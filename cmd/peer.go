package cmd

import (
	"github.com/milquellc/codis/p2p"
	"github.com/spf13/viper"

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

			peer := p2p.NewPeer(ctx, cfg)

			if err := peer.AdvertiseConnect(ctx, cfg.Rendezvous); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("peer advertised itself at the %s rendezvous point", cfg.Rendezvous)
			}

			go func() {
				if err := peer.StartRPCServer(); err != nil {
					logger.Error(err)
				}
			}()

			logger.Info("host is running! listening at %s", peer.ListenAddrs())
			peer.RunUntilCancel()
		},
	}

	cmd.Flags().String("client", "", "multiaddress of authorized client for this peer")
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		logger.Fatal(err)
	}

	return cmd
}
