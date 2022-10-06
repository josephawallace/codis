package p2p

import (
	"codis/pkg/keygen"
	"codis/pkg/log"
	"codis/pkg/utils"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

type Peer struct {
	host      libhost.Host
	kdht      *dht.IpfsDHT
	discovery *drouting.RoutingDiscovery
}

func NewPeer(bootstraps []string, psk string, keyID string) *Peer {
	logger := log.NewLogger()

	ctx := logger.WithCtx(context.Background())
	logger.Debug("Created context with attached logger.")

	bootstrapAddrs, err := utils.StringsToAddrs(bootstraps)
	if err != nil {
		logger.Fatal(err)
	}

	host, kdht, err := setupHostAndDHT(ctx, bootstrapAddrs, pnet.PSK(psk), keyID)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Debug("Created libp2p host that is using Kademlia DHT for peer routing.")

	if err = kdht.Bootstrap(ctx); err != nil {
		logger.Fatal(err)
	}
	logger.Debug("Successfully put KDHT in a bootstrapping state.")

	bootstrapInfos, err := utils.AddrsToInfos(bootstrapAddrs)
	if err != nil {
		logger.Fatal(err)
	}

	peersConnected := bootstrapDHT(ctx, host, bootstrapInfos)
	logger.Debug("Connected to %d/%d bootstrap peers.", peersConnected, len(bootstrapInfos))

	routingDiscovery := drouting.NewRoutingDiscovery(kdht)
	logger.Debug("Created peer discovery service.")

	return &Peer{
		host:      host,
		kdht:      kdht,
		discovery: routingDiscovery,
	}
}

func (p *Peer) AdvertiseConnect(ctx context.Context, rendezvous string) error {
	_, err := p.discovery.Advertise(ctx, rendezvous)
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 5) // give time for advertisement to propagate

	peerCh, err := p.discovery.FindPeers(ctx, rendezvous)
	if err != nil {
		return err
	}

	go handlePeerDiscovery(ctx, p.host, peerCh)

	return nil
}

func (p *Peer) StartRPCServer() error {
	keygenService := keygen.NewKeygenService(p.host)
	rpcHost := gorpc.NewServer(p.host, "/codis/p2p/rpc/keygen")
	if err := rpcHost.Register(keygenService); err != nil {
		return err
	}
	for {
		time.Sleep(time.Second * 1)
	}
}

func (p *Peer) StartRPCClient(ctx context.Context, host string) (*gorpc.Client, error) {
	_ = keygen.NewKeygenService(p.host)

	hostAddr, err := multiaddr.NewMultiaddr(host)
	if err != nil {
		return nil, err
	}

	hostInfo, err := peer.AddrInfoFromP2pAddr(hostAddr)
	if err != nil {
		return nil, err
	}

	if err = p.host.Connect(ctx, *hostInfo); err != nil {
		return nil, err
	}

	rpcClient := gorpc.NewClient(p.host, "/codis/p2p/rpc/keygen")
	return rpcClient, nil
}

func (p *Peer) RunUntilCancel() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\nReceived signal, shutting down...")
	if err := p.host.Close(); err != nil {
		panic(err)
	}
}

func setupHostAndDHT(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK, keyID string) (libhost.Host, *dht.IpfsDHT, error) {
	privKey, err := utils.GetOrCreatePrivKey(keyID)
	if err != nil {
		return nil, nil, err
	}

	listenAddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return nil, nil, err
	}

	bootstrapInfos, err := utils.AddrsToInfos(bootstrapAddrs)
	if err != nil {
		return nil, nil, err
	}

	var kdht *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
		libp2p.PrivateNetwork(psk),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableAutoRelay(),
		libp2p.Routing(func(h libhost.Host) (routing.PeerRouting, error) {
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.BootstrapPeers(bootstrapInfos...))
			return kdht, err
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	return host, kdht, nil
}

func bootstrapDHT(ctx context.Context, host libhost.Host, bootstrapInfos []peer.AddrInfo) int {
	logger := log.Ctx(ctx)
	peersConnected := 0
	var wg sync.WaitGroup
	for _, info := range bootstrapInfos {
		wg.Add(1)
		p := info
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, p); err != nil {
				logger.Error(err)
			} else {
				logger.Info("Connected to peer %s.", p.ID.String())
				peersConnected++
			}
		}()
	}
	wg.Wait()

	return peersConnected
}

func handlePeerDiscovery(ctx context.Context, host libhost.Host, peerCh <-chan peer.AddrInfo) {
	logger := log.Ctx(ctx)
	for info := range peerCh {
		if info.ID == host.ID() {
			continue
		}
		err := host.Connect(ctx, info)
		if err != nil {
			logger.Error(err)
		} else {
			host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
			logger.Info("Connected to peer %s", info.ID)
		}
	}
}
