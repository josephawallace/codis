package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	config, err := ParseArgs()
	if err != nil {
		log.Panicf("Error occurred in ParseArgs: %+v\n")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ip := os.Getenv("IP")
	port := os.Getenv("PORT")
	seed, _ := strconv.Atoi(os.Getenv("SEED"))
	psk, _ := hex.DecodeString(os.Getenv("PSK"))
	node, err := NewHost(ip, port, int64(seed), psk)
	if err != nil {
		log.Panicf("Error occurred in newHost: %+v", err)
	}
	nodeInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	nodeAddrs, _ := peer.AddrInfoToP2pAddrs(&nodeInfo)
	log.Printf("Host ID: %+v\n", node.ID().String())
	log.Printf("Host listening at: %+v\n", nodeAddrs)

	kademliaDHT, err := dht.New(ctx, node)
	if err != nil {
		log.Panicf("Error occurred in dht.New: %+v\n", err)
	}

	var wg sync.WaitGroup
	for _, peerAddr := range config.PeerAddrs {
		peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			log.Printf("Error occurred in peer.AddrInfoFromP2pAddr: %+v\n", err)
		}
		node.Peerstore().AddAddrs(peerAddrInfo.ID, peerAddrInfo.Addrs, peerstore.PermanentAddrTTL)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := node.Connect(ctx, *peerAddrInfo); err != nil {
				log.Printf("Error occurred in host.Connect: %+v\n", err)
			} else {
				log.Printf("Connection established with peer: %+v\n", *peerAddrInfo)
			}
		}()
		wg.Wait()

		//_ = keygenService.Keygen(ctx, peerAddrInfo.ID)
	}

	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, config.Rendezvous)

	peerCh, err := routingDiscovery.FindPeers(ctx, config.Rendezvous)
	if err != nil {
		log.Panicf("Error occurred in routingDiscovery.FindPeers: %+v\n")
	}

	for peerNode := range peerCh {
		if peerNode.ID == node.ID() {
			continue
		}
		log.Printf("Found peer: %+v\n", peerNode)
	}

	run(node, cancel)
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

func NewHost(ip string, port string, seed int64, psk pnet.PSK) (host.Host, error) {
	var reader io.Reader
	if seed == 0 {
		reader = rand.Reader
	} else {
		reader = mrand.New(mrand.NewSource(seed))
	}

	privKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, reader)
	if err != nil {
		return nil, err
	}

	nodeMultiaddr, err := multiaddr.NewMultiaddr("/ip4/" + ip + "/tcp/" + port)
	if err != nil {
		return nil, err
	}
	node, err := libp2p.New(
		libp2p.ListenAddrStrings(nodeMultiaddr.String()),
		libp2p.Identity(privKey),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Routing(NewPeerRouting),
		libp2p.PrivateNetwork(psk),
	)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func NewPeerRouting(h host.Host) (routing.PeerRouting, error) {
	ctx := context.Background()
	kaddht, err := dht.New(
		ctx,
		h,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
	)
	return kaddht, err
}
