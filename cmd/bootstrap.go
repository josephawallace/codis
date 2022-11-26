package cmd

import (
	"codis/p2p"

	"context"

	"github.com/spf13/cobra"
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
			ctx := context.Background()

			bootstrap := p2p.NewPeer(ctx, cfg)

			logger.Info("bootstrap is running! listening at %s", bootstrap.ListenAddrs())
			bootstrap.RunUntilCancel()
		},
	}

	return cmd
}
