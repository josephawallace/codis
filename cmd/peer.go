package cmd

import (
	"context"
	"github.com/milquellc/codis/network"
	"github.com/milquellc/codis/protocols"

	"github.com/spf13/cobra"
)

// startPeerCmd starts a "regular" node. This node reaches out to the given bootstrap nodes to learn about the network.
// After discovering the peers on the network, the node can participate in the cryptographic protocols.
func startPeerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Starts a new peer",
		Long:  `Creates a new peer that connects to bootstrap nodes.`,
		Run: func(c *cobra.Command, args []string) {
			ctx := context.Background()

			peer := network.NewPeer(ctx, cfg)

			if err := peer.AdvertiseConnect(ctx, cfg.Rendezvous); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("peer advertised itself at the %s rendezvous point", cfg.Rendezvous)
			}

			keygenService := protocols.NewKeygenService(peer.Host)
			if err := peer.Subscribe(ctx, protocols.KeygenSubscription, protocols.KeygenSubscriptionHandler, keygenService); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("subscribed to keygen pubsub channel")
			}

			signService := protocols.NewSignService(peer.Host)
			if err := peer.Subscribe(ctx, protocols.SignSubscription, protocols.SignSubscriptionHandler, signService); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("subscribed to sign pubsub channel")
			}

			go func() {
				if err := peer.StartRPCServer(keygenService, signService); err != nil {
					logger.Error(err)
				}
			}()

			logger.Info("host is running! listening at %s", peer.ListenAddrs())
			peer.RunUntilCancel()
		},
	}

	return cmd
}
