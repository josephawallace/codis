package cmd

import (
	"codis/p2p"
	"context"

	"github.com/spf13/cobra"
)

// startPeerCmd starts a "regular" node. This node reaches out to the given bootstrap nodes to learn about the p2p.
// After discovering the peers on the p2p, the node can participate in the cryptographic protocols.
func startPeerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Starts a new peer",
		Long:  `Creates a new peer that connects to bootstrap nodes.`,
		Run: func(c *cobra.Command, args []string) {
			ctx := context.Background()
			peer := p2p.NewPeer(ctx, cfg.Peers[peerCfgId])
			rendezvous := "1ae697d4da3b45469e81ef80dba7ad40"

			if err := peer.AdvertiseConnect(ctx, rendezvous); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("peer advertised itself at the %s rendezvous point", rendezvous)
			}

			go func() {
				if err := peer.StartRPCServer(); err != nil {
					logger.Error(err)
				}
			}()

			logger.Info("host is running! Listening at %s", peer.ListenAddrs())
			peer.RunUntilCancel()
		},
	}

	return cmd
}
