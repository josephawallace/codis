package main

import (
	"context"
	"crypto/rand"
	"io"
	mrand "math/rand"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

func ConnectToPeers(ctx context.Context, node host.Host, peerAddrs []multiaddr.Multiaddr) (int, error) {
	numPeerConnections := 0
	for _, peerAddr := range peerAddrs {
		peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			return numPeerConnections, err
		}
		node.Peerstore().AddAddrs(peerAddrInfo.ID, peerAddrInfo.Addrs, peerstore.PermanentAddrTTL)
		if err = node.Connect(ctx, *peerAddrInfo); err != nil {
			return numPeerConnections, err
		}
		numPeerConnections++
	}
	return numPeerConnections, nil
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
