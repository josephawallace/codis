package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
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
	node, err := NewHost(ip, port, int64(seed))
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

	keygenService := NewKeygenService(node) // set stream handlers, set every service available here.
	if len(config.PeerAddrs) > 0 {
		numPeerConnections, err := ConnectToPeers(ctx, node, config.PeerAddrs) // only dialing peers, not setting up streams
		if err != nil {
			log.Printf("Error occurred in ConnectToPeers: %+v\n", err)
		}
		log.Printf("Number of peer connections: %d\n", numPeerConnections)

		if config.ProtocolID == ID {
			for _, peerAddr := range config.PeerAddrs {
				peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
				if err != nil {
					log.Printf("Error occurred in peer.AddrInfoFromP2pAddr: %+v\n", err)
				}
				_ = keygenService.Keygen(ctx, peerAddrInfo.ID)
			}
		}
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
