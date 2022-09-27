package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
	mrand "math/rand"
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
		if config.ProtocolID == ID {
			for _, peerAddr := range config.PeerAddrs {
				peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
				node.Peerstore().AddAddrs(peerAddrInfo.ID, peerAddrInfo.Addrs, peerstore.PermanentAddrTTL)
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

func NewHost(ip string, port string, seed int64) (host.Host, error) {
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
	)
	if err != nil {
		return nil, err
	}

	return node, nil
}
