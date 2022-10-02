package p2p

import (
	"codis/pkg/keygen"
	"codis/pkg/utils"
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type P2P struct {
	Ctx        context.Context
	Host       libhost.Host
	Kdht       *dht.IpfsDHT
	Discovery  *drouting.RoutingDiscovery
	Rendezvous string
}

func NewP2P(bootstrapAddrs []multiaddr.Multiaddr, rendezvous string, psk pnet.PSK) *P2P {
	ctx := context.Background()

	host, kdht := setupHostAndDHT(ctx, bootstrapAddrs, psk)
	log.Printf("Created host and Kademlia DHT")

	bootstrapDHT(ctx, host, kdht, bootstrapAddrs)
	log.Printf("Bootstrapped Kademlia DHT")

	routingDiscovery := drouting.NewRoutingDiscovery(kdht)
	log.Printf("Created peer discovery service")

	p2p := &P2P{
		Ctx:        ctx,
		Host:       host,
		Kdht:       kdht,
		Discovery:  routingDiscovery,
		Rendezvous: rendezvous,
	}

	return p2p
}

func (p2p *P2P) AdvertiseConnect() {
	ttl, err := p2p.Discovery.Advertise(p2p.Ctx, p2p.Rendezvous)
	if err != nil {
		log.Fatalf("Failed to advertise services: %+v", err)
	}
	log.Printf("Advertised the %+v service", p2p.Rendezvous)
	time.Sleep(time.Second * 5) // give time for advertisement to propagate
	log.Printf("Service TTL is %+v", ttl)

	peerCh, err := p2p.Discovery.FindPeers(p2p.Ctx, p2p.Rendezvous)
	if err != nil {
		log.Fatalf("Peer discovery failed: %+v", err)
	}
	log.Printf("Discovered service peers")

	go handlePeerDiscovery(p2p.Host, peerCh)
	log.Printf("Started peer connection handler")
}

func (p2p *P2P) StartRPCServer() {
	keygenService := keygen.NewKeygenService(p2p.Host)
	rpcHost := gorpc.NewServer(p2p.Host, "/codis/p2p/rpc/keygen")
	if err := rpcHost.Register(keygenService); err != nil {
		log.Fatalf("Failed to register keygen service: %+v\n", err)
	}
	log.Printf("Started RPC server")

	for {
		time.Sleep(time.Second * 1)
	}
}

func (p2p *P2P) StartRPCClient(host multiaddr.Multiaddr) *gorpc.Client {
	_ = keygen.NewKeygenService(p2p.Host)
	hostInfo, err := peer.AddrInfoFromP2pAddr(host)
	if err != nil {
		log.Fatalf("Failed to parse host address: %+v\n", err)
	}

	// In this case, the Host on p2p is acting as the client node.
	if err = p2p.Host.Connect(p2p.Ctx, *hostInfo); err != nil {
		log.Fatalf("Failed to connect to host %s: %+v\n", hostInfo.ID, err)
	}

	return gorpc.NewClient(p2p.Host, "/codis/p2p/rpc/keygen")
}

func (p2p *P2P) RunUntilCancel() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\nReceived signal, shutting down...")

	// shut the node down
	if err := p2p.Host.Close(); err != nil {
		panic(err)
	}
}

func setupHostAndDHT(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK) (libhost.Host, *dht.IpfsDHT) {
	privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatalf("Failed to generate private key: %+v\n", err)
	}

	listenAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

	var kdht *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
		libp2p.PrivateNetwork(psk),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableAutoRelay(),
		libp2p.Routing(func(h libhost.Host) (routing.PeerRouting, error) {
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.BootstrapPeers(utils.AddrsToInfos(bootstrapAddrs)...))
			return kdht, err
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create the host: %+v\n", err)
	}

	hostInfo := peer.AddrInfo{
		ID:    host.ID(),
		Addrs: host.Addrs(),
	}
	hostAddrs, _ := peer.AddrInfoToP2pAddrs(&hostInfo)
	log.Printf("Host ID: %+v\n", host.ID().String())
	log.Printf("Host listening at: %+v\n", hostAddrs)

	return host, kdht
}

func bootstrapDHT(ctx context.Context, host libhost.Host, kdht *dht.IpfsDHT, bootstrapAddrs []multiaddr.Multiaddr) {
	if err := kdht.Bootstrap(ctx); err != nil {
		log.Fatalf("Failed to Bootstrap the Kademlia! %v", err)
	}
	log.Printf("Kademlia DHT set to Bootstrap Mode")

	var wg sync.WaitGroup
	bootstrapInfos := utils.AddrsToInfos(bootstrapAddrs)
	numConnectedPeers := 0
	for _, info := range bootstrapInfos {
		p := info
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, p); err != nil {
				log.Printf("Failed to connect to peer: %+v\n", err)
			} else {
				numConnectedPeers++
			}
		}()
	}
	wg.Wait()
	log.Printf("Connected to %d out of %d bootstrap peers", numConnectedPeers, len(bootstrapAddrs))
}

func handlePeerDiscovery(host libhost.Host, peerCh <-chan peer.AddrInfo) {
	for info := range peerCh {
		if info.ID == host.ID() {
			continue
		}

		err := host.Connect(context.Background(), info)
		if err != nil {
			log.Printf("Failed to connect to peer %+v", info.String())
		} else {
			log.Printf("Connected to peer %v", info.String())
		}
	}
}
