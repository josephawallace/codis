package p2p

import (
	"codis/configs"
	"codis/log"
	"codis/protocols"
	"codis/utils"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libhost "github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

type Peer struct {
	Host libhost.Host

	kdht      *dht.IpfsDHT
	discovery *drouting.RoutingDiscovery
	logger    *log.Logger
}

func NewPeer(ctx context.Context, peerCfg configs.Peer) *Peer {
	logger := log.NewLogger()

	bootstrapAddrs, err := utils.AddrStringsToAddrs(peerCfg.Bootstraps)
	if err != nil {
		logger.Fatal(err)
	}

	pskBytes, err := hex.DecodeString(peerCfg.Network.PSK)
	if err != nil {
		logger.Fatal(err)
	}

	host, kdht, err := setupHostAndDHT(ctx, bootstrapAddrs, pskBytes, peerCfg)
	if err != nil {
		logger.Fatal(err)
	} else {
		logger.Debug("created libp2p host, using kademlia dht for peer routing")
	}

	if err = kdht.Bootstrap(ctx); err != nil {
		logger.Fatal(err)
	} else {
		logger.Debug("successfully put kdht in a bootstrapping state")
	}

	bootstrapInfos, err := utils.AddrsToInfos(bootstrapAddrs)
	if err != nil {
		logger.Fatal(err)
	}

	peersConnected := bootstrapDHT(ctx, host, bootstrapInfos, logger)
	logger.Debug("connected to %d/%d bootstrap peers", peersConnected, len(bootstrapInfos))

	routingDiscovery := drouting.NewRoutingDiscovery(kdht)
	logger.Debug("created peer discovery service")

	return &Peer{
		Host: host,

		kdht:      kdht,
		discovery: routingDiscovery,
		logger:    logger,
	}
}

func (p *Peer) ListenAddrs() string {
	info := libhost.InfoFromHost(p.Host)
	addrs, err := libpeer.AddrInfoToP2pAddrs(info)
	if err != nil {
		p.logger.Error(err)
		return ""
	}

	addrStrs := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addrStrs = append(addrStrs, addr.String())
	}
	return strings.Join(addrStrs, ",")
}

func (p *Peer) AdvertiseConnect(ctx context.Context, rendezvous string) error {
	_, err := p.discovery.Advertise(ctx, rendezvous)
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 5) // give time for advertisement to propagate

	peerCh, err := p.discovery.FindPeers(ctx, rendezvous)
	if err != nil {
		return err
	}

	go handlePeerDiscovery(ctx, p.Host, peerCh, p.logger)

	return nil
}

func (p *Peer) StartRPCServer(clientAddr string) error {
	fromClientOnly := func(pid libpeer.ID, name string, method string) bool {
		clientInfo, err := libpeer.AddrInfoFromString(clientAddr)
		if err != nil {
			p.logger.Error(err)
			return false
		}
		return pid.String() == clientInfo.ID.String()
	}
	rpcHost := gorpc.NewServer(p.Host, "/codis/rpc", gorpc.WithAuthorizeFunc(fromClientOnly))

	keygenService := protocols.NewKeygenService(p.Host)
	signService := protocols.NewSignService(p.Host)

	if err := rpcHost.Register(keygenService); err != nil {
		return err
	}
	p.logger.Debug("registered keygen service")

	if err := rpcHost.Register(signService); err != nil {
		return err
	}
	p.logger.Debug("registered sign service")

	for {
		time.Sleep(time.Second * 1)
	}
}

func (p *Peer) StartRPCClient(ctx context.Context, hostAddr string) (*gorpc.Client, error) {
	_ = protocols.NewKeygenService(p.Host)

	hostInfo, err := libpeer.AddrInfoFromString(hostAddr)
	if err != nil {
		return nil, err
	}

	if err = p.Host.Connect(ctx, *hostInfo); err != nil {
		return nil, err
	}

	rpcClient := gorpc.NewClient(p.Host, "/codis/rpc")
	return rpcClient, nil
}

func (p *Peer) RunUntilCancel() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\nReceived signal, shutting down...")
	if err := p.Host.Close(); err != nil {
		panic(err)
	}
}

func setupHostAndDHT(ctx context.Context, bootstrapAddrs []multiaddr.Multiaddr, psk pnet.PSK, peerCfg configs.Peer) (libhost.Host, *dht.IpfsDHT, error) {
	privKey, err := utils.GetOrCreatePrivKey(peerCfg.ID)
	if err != nil {
		return nil, nil, err
	}

	listenAddr, err := multiaddr.NewMultiaddr("/ip4/" + peerCfg.IP + "/tcp/" + peerCfg.Port)
	if err != nil {
		return nil, nil, err
	}

	bootstrapInfos, err := utils.AddrsToInfos(bootstrapAddrs)
	if err != nil {
		return nil, nil, err
	}

	var kdht *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
		libp2p.PrivateNetwork(psk),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableAutoRelay(),
		libp2p.Routing(func(h libhost.Host) (routing.PeerRouting, error) {
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.BootstrapPeers(bootstrapInfos...))
			return kdht, err
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	return host, kdht, nil
}

func bootstrapDHT(ctx context.Context, host libhost.Host, bootstrapInfos []libpeer.AddrInfo, logger *log.Logger) int {
	peersConnected := 0
	var wg sync.WaitGroup
	for _, info := range bootstrapInfos {
		wg.Add(1)
		p := info
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, p); err != nil {
				logger.Error(err)
			} else {
				logger.Info("connected to bootstrap node %s", p.ID.String())
				peersConnected++
			}
		}()
	}
	wg.Wait()

	return peersConnected
}

func handlePeerDiscovery(ctx context.Context, host libhost.Host, peerCh <-chan libpeer.AddrInfo, logger *log.Logger) {
	for info := range peerCh {
		if info.ID == host.ID() {
			continue
		}
		err := host.Connect(ctx, info)
		if err != nil {
			logger.Error(err)
		} else {
			host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
			logger.Info("connected to peer %s", info.ID)
		}
	}
}
