package mobile

import (
	"context"
	"github.com/milquellc/codis/configs"
	"github.com/milquellc/codis/log"
	"github.com/milquellc/codis/network"
	"github.com/milquellc/codis/protocols"
	"github.com/spf13/cobra"
)

type Client struct {
	peer   *network.Peer
	logger *log.Logger
}

func NewClient() *Client {
	ctx := context.Background()
	cfg := configs.NewConfig(&cobra.Command{})
	peer := network.NewPeer(ctx, cfg)
	logger := log.NewLogger()

	return &Client{
		peer:   peer,
		logger: logger,
	}
}

func (c *Client) GetServicePeerList() []string {
	var servicePeerList []string
	for _, peerId := range c.peer.Host.Peerstore().Peers() {
		if peerId.String() == c.peer.Host.ID().String() {
			continue // don't self-dial
		}

		// service peers must implement the protocols checked below
		if pid, err := c.peer.Host.Peerstore().FirstSupportedProtocol(peerId, protocols.KeygenPId); err != nil || pid != protocols.KeygenPId {
			continue
		}
		if pid, err := c.peer.Host.Peerstore().FirstSupportedProtocol(peerId, protocols.SignPId); err != nil || pid != protocols.KeygenPId {
			continue
		}
		servicePeerList = append(servicePeerList, peerId.String())
	}
	return servicePeerList
}
