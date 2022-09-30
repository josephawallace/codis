package main

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"
)

func addrsToInfos(multiaddrs []multiaddr.Multiaddr) []peer.AddrInfo {
	infos := make([]peer.AddrInfo, 0, len(multiaddrs))
	for _, addr := range multiaddrs {
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Printf("Failed to convert multiaddr to %+v to peer addrinfo: %+v", addr.String(), err)
			continue
		}
		infos = append(infos, *info)
	}
	return infos
}
