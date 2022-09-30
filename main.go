package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p/core/host"
)

func main() {
	_, cancel := context.WithCancel(context.Background())

	config, err := ParseArgs()
	if err != nil {
		log.Panicf("Error occurred in ParseArgs: %+v\n", err)
	}

	psk, _ := hex.DecodeString(os.Getenv("PSK"))
	participant := NewParticipant(config.Rendezvous, config.BootstrapAddrs, psk)
	if len(config.BootstrapAddrs) > 0 {
		participant.AdvertiseConnect()
		participant.StartKeygen(participant.Node.Peerstore().Peers()[0])
	}

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
