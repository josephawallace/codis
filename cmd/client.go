package cmd

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"

	"codis/pkg/keygen"
	"codis/pkg/p2p"
	"codis/proto/pb"

	"github.com/spf13/cobra"
)

func startClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Starts a test client",
		Long: `The Codis stack involves client nodes to send RPC commands to the server nodes. The client nodes are written
in Typescript, in order to leverage the slew of Node.js cryptocurrency libraries. This makes testing client commands less
convenient though, which is why we have this--to send test client commands from one-off client nodes.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			client := p2p.NewPeer(ctx, cfg.Peer.Bootstraps, cfg.Network.PSK, cfg.Peer.KeyID)

			host, err := cmd.Flags().GetString("host")
			if err != nil {
				logger.Fatal(err)
			}
			rpcClient, err := client.StartRPCClient(ctx, host)
			if err != nil {
				logger.Fatal(err)
			}
			logger.Debug("Started RPC client.")

			protocol, err := cmd.Flags().GetString("protocol")
			if err != nil {
				logger.Fatal(err)
			}
			peers, err := cmd.Flags().GetStringSlice("peers")
			if err != nil {
				logger.Fatal(err)
			}
			if protocol == keygen.ID {
				keygenArgs := pb.KeygenArgs{Count: 4, Threshold: 2, Ids: peers}
				keygenReply := pb.KeygenReply{}

				if err = rpcClient.Call(peer.ID(host), "KeygenService", "Keygen", &keygenArgs, &keygenReply); err != nil {
					logger.Fatal(err)
				}
			}

			logger.Info("Client is running! Listening at %s.", client.ListenAddrs)

			client.RunUntilCancel()
		},
	}

	cmd.Flags().String("protocol", "", "select the protocol you want your host to start")
	if err := cmd.MarkFlagRequired("protocol"); err != nil {
		logger.Fatal(err)
	}
	cmd.Flags().StringSlice("peers", []string{}, "peers that should start the specified protocol")
	if err := cmd.MarkFlagRequired("peers"); err != nil {
		logger.Fatal(err)
	}
	cmd.PersistentFlags().String("host", "", "codis node for this client to use as host")
	if err := cmd.MarkPersistentFlagRequired("host"); err != nil {
		logger.Fatal(err)
	}

	return cmd
}
