package cmd

import (
	"codis/pkg/keygen"
	"codis/pkg/p2p"
	"codis/proto/pb"
	"encoding/hex"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"

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
			psk, err := hex.DecodeString(config.PSK)
			if err != nil {
				log.Fatalf("Failed to decode PSK")
			}
			p2phost := p2p.NewP2P([]multiaddr.Multiaddr{}, config.Rendezvous, psk)

			hostAddrStr, _ := cmd.Flags().GetString("host")
			hostAddr, err := multiaddr.NewMultiaddr(hostAddrStr)
			if err != nil {
				log.Fatalf("Failed to parse host address: %+v\n", err)
			}
			hostInfo, err := peer.AddrInfoFromP2pAddr(hostAddr)
			if err != nil {
				log.Fatalf("Failed to get info from host address: %+v\n", err)
			}

			rpcClient := p2phost.StartRPCClient(hostAddr)
			if protocol, _ := cmd.Flags().GetString("protocol"); protocol == keygen.ID {
				peerIDs, _ := cmd.Flags().GetStringSlice("peers")
				log.Printf("%s\n", peerIDs)
				keygenArgs := pb.KeygenArgs{Count: 3, Threshold: 5, Ids: peerIDs}
				keygenReply := pb.KeygenReply{}

				if err := rpcClient.Call(hostInfo.ID, "KeygenService", "Keygen", &keygenArgs, &keygenReply); err != nil {
					log.Fatalf("RPC call failed: %+v\n", err)
				}
			}

			p2phost.RunUntilCancel()
		},
	}

	cmd.Flags().String("protocol", "", "select the protocol you want your host to start")
	if err := cmd.MarkFlagRequired("protocol"); err != nil {
		log.Fatalf("Failed to set flag as required: %+v", err)
	}
	cmd.Flags().StringSlice("peers", []string{}, "peers that should start the specified protocol")
	if err := cmd.MarkFlagRequired("peers"); err != nil {
		log.Fatalf("Failed to set flag as required: %+v\n", err)
	}
	cmd.PersistentFlags().String("host", "", "codis node for this client to use as host")
	if err := cmd.MarkPersistentFlagRequired("host"); err != nil {
		log.Fatalf("Failed to set flag as required: %+v", err)
	}

	return cmd
}
