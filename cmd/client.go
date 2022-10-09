package cmd

import (
	"codis/pkg/p2p"
	"codis/pkg/protocols"
	"context"
	"github.com/multiformats/go-multiaddr"

	"codis/proto/pb"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
)

var (
	protocol      string
	party         []string
	host          string
	protocolArgs  interface{}
	protocolReply interface{}
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
			client := p2p.NewPeer(ctx, cfg.Peers[peerCfgId])

			rpcClient, err := client.StartRPCClient(ctx, host)
			if err != nil {
				logger.Fatal(err)
			} else {
				logger.Debug("started RPC client")
			}

			hostAddr, err := multiaddr.NewMultiaddr(host)
			if err != nil {
				logger.Fatal(err)
			}
			hostInfo, err := peer.AddrInfoFromP2pAddr(hostAddr)
			if err != nil {
				logger.Fatal(err)
			}

			if protocol == protocols.KeygenPID {
				protocolArgs = pb.KeygenArgs{Count: 4, Threshold: 2, Ids: peers}
				protocolReply = pb.KeygenReply{}

				if err = rpcClient.Call(hostInfo.ID, "KeygenService", "Keygen", &protocolArgs, &protocolReply); err != nil {
					logger.Fatal(err)
				}
			}

			logger.Info("client is running! listening at %s", client.ListenAddrs())

			client.RunUntilCancel()
		},
	}

	cmd.Flags().StringVarP(&protocol, "protocol", "p", "", "protocol for host to run")
	if err := cmd.MarkFlagRequired("protocol"); err != nil {
		logger.Fatal(err)
	}
	cmd.Flags().StringSliceVarP(&party, "party", "P", []string{}, "peers involved in the protocol")
	if err := cmd.MarkFlagRequired("peers"); err != nil {
		logger.Fatal(err)
	}
	cmd.PersistentFlags().StringVarP(&host, "host", "H", "", "peer used as host to communicate with other peers ")
	if err := cmd.MarkPersistentFlagRequired("host"); err != nil {
		logger.Fatal(err)
	}

	return cmd
}
