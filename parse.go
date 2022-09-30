package main

import (
	"flag"
	"github.com/multiformats/go-multiaddr"
	"strings"
)

type addrList []multiaddr.Multiaddr

func (al *addrList) String() string {
	strs := make([]string, len(*al))
	for i, addr := range *al {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ",")
}

func (al *addrList) Set(value string) error {
	addr, err := multiaddr.NewMultiaddr(value)
	if err != nil {
		return err
	}
	*al = append(*al, addr)
	return nil
}

func StringsToAddrs(addrStrings []string) (maddrs []multiaddr.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

type Config struct {
	ListenAddrs    addrList
	BootstrapAddrs addrList
	ProtocolID     string
	Rendezvous     string
}

func ParseArgs() (Config, error) {
	config := Config{}
	flag.Var(&config.ListenAddrs, "listen", "Multiaddrs of the host created from an instance")
	flag.Var(&config.BootstrapAddrs, "bootstrap", "Multiaddrs of bootstrap peers on the network")
	flag.StringVar(&config.ProtocolID, "protocol", "", "Protocol the peers should execute")
	flag.StringVar(&config.Rendezvous, "rendezvous", "rendezvous-string", "Protocol the peers should execute")
	flag.Parse()

	return config, nil
}
