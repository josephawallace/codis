package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
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

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Participant struct {
	Ctx       context.Context
	Node      host.Host
	Kdht      *dht.IpfsDHT
	Discovery *drouting.RoutingDiscovery
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := ParseArgs()
	if err != nil {
		log.Panicf("Error occurred in ParseArgs: %+v\n")
	}
	psk, _ := hex.DecodeString(os.Getenv("PSK"))

	participant := NewParticipant(ctx, config.BootstrapAddrs, psk)
	run(participant.Node, cancel)
}

func run(h host.Host, cancel func()) {
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	<-c

	fmt.Printf("\rExiting...\n")

	cancel()

	if err := h.Close(); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func NewParticipant(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK) *Participant {
	node, kdht := setupNodeAndDHT(ctx, bootstrapAddrs, psk)
	log.Printf("Created node and Kademlia DHT")

	bootstrapDHT(ctx, node, kdht, bootstrapAddrs)
	log.Printf("Bootstrapped Kademlia DHT")

	routingDiscovery := drouting.NewRoutingDiscovery(kdht)
	log.Printf("Created peer discovery service")

	participant := &Participant{
		Ctx:       ctx,
		Node:      node,
		Kdht:      kdht,
		Discovery: routingDiscovery,
	}

	return participant
}

func setupNodeAndDHT(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK) (host.Host, *dht.IpfsDHT) {
	privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatalf("Failed to generate private key: %+v\n", err)
	}

	listenAddr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

	var kdht *dht.IpfsDHT
	node, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
		libp2p.PrivateNetwork(psk),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableAutoRelay(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.BootstrapPeers(addrsToInfos(bootstrapAddrs)...))
			return kdht, err
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create the host: %+v\n", err)
	}

	hostInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	hostAddrs, _ := peer.AddrInfoToP2pAddrs(&hostInfo)
	log.Printf("Host ID: %+v\n", node.ID().String())
	log.Printf("Host listening at: %+v\n", hostAddrs)

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
