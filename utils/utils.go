package utils

import (
	"codis/configs"
	b58 "github.com/mr-tron/base58"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// AddrsToInfos converts a multiaddr slice into a addrinfo slice
func AddrsToInfos(multiaddrs []multiaddr.Multiaddr) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(multiaddrs))
	for _, addr := range multiaddrs {
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// AddrStringsToAddrs converts a slice containing string representations of multiaddrs into a slice containing multiaddr objects
// from libp2p
func AddrStringsToAddrs(addrs []string) ([]multiaddr.Multiaddr, error) {
	multiaddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		newAddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		multiaddrs = append(multiaddrs, newAddr)
	}
	return multiaddrs, nil
}

func AddrsToAddrStrings(multiaddrs []multiaddr.Multiaddr) []string {
	addrStrs := make([]string, 0, len(multiaddrs))
	for _, addr := range multiaddrs {
		addrStr := addr.String()
		addrStrs = append(addrStrs, addrStr)
	}
	return addrStrs
}

func AddrStringsToPeerIds(addrs []string) ([]peer.ID, error) {
	peerIds := make([]peer.ID, 0, len(addrs))
	for _, addr := range addrs {
		info, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		peerIds = append(peerIds, info.ID)
	}
	return peerIds, nil
}

func PeerIdStringsToPeerIds(peerIdStrs []string) ([]peer.ID, error) {
	peerIds := make([]peer.ID, 0, len(peerIdStrs))
	for _, peerIdStr := range peerIdStrs {
		peerIdBytes, err := b58.Decode(peerIdStr)
		if err != nil {
			return nil, err
		}

		peerId, err := peer.IDFromBytes(peerIdBytes)
		if err != nil {
			return nil, err
		}
		peerIds = append(peerIds, peerId)
	}
	return peerIds, nil
}

// AddrStringsToInfos converts a slice containing string representations of multiaddrs into a slice containing addr infos,
// without an intermediate function as a step
func AddrStringsToInfos(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, addrStr := range addrs {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return nil, err
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// GetOrCreatePrivKey will look into the `keys` directory for a keyfile that corresponds to the key ID passed as a parameter.
// This way, the addresses of the peers can remain consistent after they've been initialized. This is most useful for
// debugging purposes.
func GetOrCreatePrivKey(id string) (crypto.PrivKey, error) {
	var privKey crypto.PrivKey

	keyDir, _ := filepath.Abs("keys/")
	keyFile, _ := filepath.Abs(keyDir + "/privkey_" + id + ".dat")

	if data, err := ioutil.ReadFile(keyFile); err == nil { // key specified and data already exists
		privKey, err = crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, err
		}
	} else { // no key data found
		if privKey, _, err = crypto.GenerateKeyPair(crypto.RSA, 2048); err != nil {
			return nil, err
		}
		if id == configs.DefaultPeer.ID { // don't save key unless given explicit peer name
			return privKey, nil
		}
		if err = os.MkdirAll(keyDir, fs.ModePerm); err != nil {
			return nil, err
		}
		if data, err = crypto.MarshalPrivateKey(privKey); err != nil {
			return nil, err
		} else {
			if err = ioutil.WriteFile(keyFile, data, fs.ModePerm); err != nil {
				return nil, err
			}
		}
	}
	return privKey, nil
}
