package utils

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"
)

func AddrsToInfos(multiaddrs []multiaddr.Multiaddr) []peer.AddrInfo {
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

func StringsToAddrs(addrs []string) []multiaddr.Multiaddr {
	multiaddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		newAddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			log.Printf("Failed to convert string to %+v to multiaddr: %+v", addr, err)
			continue
		}
		multiaddrs = append(multiaddrs, newAddr)
	}

	return multiaddrs
}
