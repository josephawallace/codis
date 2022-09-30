package main

import (
	"context"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"log"
	"sync"
	"time"
)

type Participant struct {
	Ctx        context.Context
	Node       host.Host
	Kdht       *dht.IpfsDHT
	Discovery  *drouting.RoutingDiscovery
	Rendezvous string

	KeygenService KeygenService
}

func NewParticipant(rendezvous string, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK) *Participant {
	ctx := context.Background()

	node, kdht := setupNodeAndDHT(ctx, bootstrapAddrs, psk)
	log.Printf("Created node and Kademlia DHT")

	bootstrapDHT(ctx, node, kdht, bootstrapAddrs)
	log.Printf("Bootstrapped Kademlia DHT")

	routingDiscovery := drouting.NewRoutingDiscovery(kdht)
	log.Printf("Created peer discovery service")

	keygenService := NewKeygenService(node)
	log.Printf("Created keygen service")

	participant := &Participant{
		Ctx:           ctx,
		Node:          node,
		Kdht:          kdht,
		Discovery:     routingDiscovery,
		Rendezvous:    rendezvous,
		KeygenService: *keygenService,
	}

	return participant
}

func (p *Participant) AdvertiseConnect() {
	ttl, err := p.Discovery.Advertise(p.Ctx, p.Rendezvous)
	if err != nil {
		log.Fatalf("Failed to advertise services: %+v", err)
	}
	log.Printf("Advertised the %+v service", p.Rendezvous)
	time.Sleep(time.Second * 5) // give time for advertisement to propagate
	log.Printf("Service TTL is %+v", ttl)

	peerCh, err := p.Discovery.FindPeers(p.Ctx, p.Rendezvous)
	if err != nil {
		log.Fatalf("Peer discovery failed: %+v", err)
	}
	log.Printf("Discovered service peers")

	go handlePeerDiscovery(p.Node, peerCh)
	log.Printf("Started peer connection handler")
}

func (p *Participant) StartKeygen(peerID peer.ID) {
	p.KeygenService.Keygen(p.Ctx, peerID)
}

func setupNodeAndDHT(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK) (host.Host, *dht.IpfsDHT) {
	privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatalf("Failed to generate private key: %+v\n", err)
	}

	listenAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

	connManager, _ := connmgr.NewConnManager(3, 3)

	var kdht *dht.IpfsDHT
	node, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
		libp2p.PrivateNetwork(psk),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableAutoRelay(),
		libp2p.ConnectionManager(connManager),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.BootstrapPeers(addrsToInfos(bootstrapAddrs)...))
			return kdht, err
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create the host: %+v\n", err)
	}

	nodeInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	nodeAddrs, _ := peer.AddrInfoToP2pAddrs(&nodeInfo)
	log.Printf("Node ID: %+v\n", node.ID().String())
	log.Printf("Node listening at: %+v\n", nodeAddrs)

	return node, kdht
}

func bootstrapDHT(ctx context.Context, node host.Host, kdht *dht.IpfsDHT, bootstrapAddrs []multiaddr.Multiaddr) {
	if err := kdht.Bootstrap(ctx); err != nil {
		log.Fatalf("Failed to Bootstrap the Kademlia! %v", err)
	}
	log.Printf("Kademlia DHT set to Bootstrap Mode")

	var wg sync.WaitGroup
	bootstrapInfos := addrsToInfos(bootstrapAddrs)
	numConnectedPeers := 0
	for _, info := range bootstrapInfos {
		p := info
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := node.Connect(ctx, p); err != nil {
				log.Printf("Failed to connect to peer: %+v\n", err)
			} else {
				numConnectedPeers++
			}
		}()
	}
	wg.Wait()
	log.Printf("Connected to %d out of %d bootstrap peers", numConnectedPeers, len(bootstrapAddrs))
}

func handlePeerDiscovery(node host.Host, peerCh <-chan peer.AddrInfo) {
	for info := range peerCh {
		if info.ID == node.ID() {
			continue
		}
		log.Printf("handlePeerDiscovery %s", info.String())

		err := node.Connect(context.Background(), info)
		if err != nil {
			log.Printf("Failed to connect to peer %+v", info.String())
		}
		log.Printf("Connected to peer %v", info.String())
	}
}
