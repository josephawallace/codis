package cmd

import (
	"codis/pkg/p2p"
	"encoding/hex"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"log"
)

// startBootstrapCmd starts a node that listens for other nodes to connect to it. When a node connects, the bootstrap node
// will add it to its list of known peers. When other peers connect to the network via the bootstrap node, it will share
// its list of peers.
func startBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Starts a bootstrap node",
		Long:  `Creates a bootstrap node that can be used for peer discovery.`,
		Run: func(cmd *cobra.Command, args []string) {
			psk, err := hex.DecodeString(config.PSK)
			if err != nil {
				log.Fatalf("Failed to decode PSK")
			}
			// Bootstrap node is identical to the other nodes on the network. The only difference is that bootstrap nodes
			// don't attempt to reach out to other nodes on starting.
			p2phost := p2p.NewP2P([]multiaddr.Multiaddr{}, config.Rendezvous, psk)
			p2phost.RunUntilCancel()
		},
	}

	return cmd
}
