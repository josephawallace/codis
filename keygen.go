package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"os"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

const (
	ID          = "/keygen/0.0.1"
	ServiceName = "codis.keygen"
)

type KeygenService struct {
	Host host.Host
}

func NewKeygenService(h host.Host) *KeygenService {
	ks := &KeygenService{h}
	h.SetStreamHandler(ID, ks.KeygenHandler)
	return ks
}

func (ks *KeygenService) KeygenHandler(s network.Stream) {
	log.Printf("KeygenHandler hit.\n")
	if err := s.Scope().SetService(ServiceName); err != nil {
		log.Printf("Failed to set service.\n")
		return
	}

	keygen(s)
}

type Result struct {
	Key   string
	Error error
}

func (ks *KeygenService) Keygen(ctx context.Context, p peer.ID) <-chan Result {
	return Keygen(ctx, ks.Host, p)
}

func keygenError(err error) chan Result {
	endCh := make(chan Result, 1)
	endCh <- Result{Error: err}
	close(endCh)
	return endCh
}

func Keygen(ctx context.Context, h host.Host, p peer.ID) <-chan Result {
	s, err := h.NewStream(ctx, p, ID)
	if err != nil {
		return keygenError(err)
	}

	if err := s.Scope().SetService(ServiceName); err != nil {
		_ = s.Reset()
		return keygenError(err)
	}

	keygen(s)

	retCh := make(chan Result, 1)
	return retCh
}

func keygen(s network.Stream) {
	stdReader := bufio.NewReader(os.Stdin)
	buffer := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// read data from stream
	go func() {
		for { // reading should go on forever in a thread
			str, err := buffer.ReadString('\n') // reads up to a new line from buffer (stream), then continues
			if err != nil {
				continue // keep trying to read from stream
			} else {
				fmt.Printf("\u001B[32m%s\u001B[0m> ", str)
			}
		}
	}()

	// write data to stream
	go func() {
		for {
			fmt.Printf("> ")
			str, err := stdReader.ReadString('\n') // reads up to a new line from stdin, then continues
			if err != nil {
				log.Printf("Error occurred in keygen: %+v\n", err)
			}
			if _, err = buffer.WriteString(fmt.Sprintf("%s", str)); err != nil {
				continue
			} else {
				_ = buffer.Flush()
			}
		}
	}()
}
