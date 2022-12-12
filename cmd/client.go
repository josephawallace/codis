package cmd

import (
	"context"
	"errors"
	"github.com/milquellc/codis/network"
	"github.com/milquellc/codis/proto/pb"
	"github.com/milquellc/codis/protocols"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"time"
)

// startClientCmd starts a client node. This node reaches out to its configured "host" via RPC call to initiate a network
// protocol.
func startClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Starts a test client",
		Long: `The Codis stack involves client nodes to send RPC commands to the server nodes. The client nodes are written
in Typescript, in order to leverage the slew of Node.js cryptocurrency libraries. This makes testing client commands less
convenient though, which is why we have this--to send test client commands from one-off client nodes.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			client := network.NewPeer(ctx, cfg)

			logger.Info("client is running! listening at %s", client.ListenAddrs())

			time.Sleep(time.Second * 5)

			var servicePeerList []string
			for _, peerId := range client.Host.Peerstore().Peers() {
				if peerId.String() == client.Host.ID().String() {
					continue // don't self-dial
				}

				// service peers must be online and implement the protocols checked below
				if _, err := client.Host.NewStream(context.Background(), peerId, "/ipfs/ping/1.0.0"); err != nil {
					continue
				}
				if pid, err := client.Host.Peerstore().FirstSupportedProtocol(peerId, protocols.KeygenPId); err != nil || pid != protocols.KeygenPId {
					continue
				}
				if pid, err := client.Host.Peerstore().FirstSupportedProtocol(peerId, protocols.SignPId); err != nil || pid != protocols.SignPId {
					continue
				}
				servicePeerList = append(servicePeerList, peerId.String())
			}

			topic, _ := cmd.Flags().GetString("topic")

			if topic == protocols.KeygenTopic {
				alg, _ := cmd.Flags().GetString("algorithm")
				count, _ := cmd.Flags().GetInt32("count")
				threshold, _ := cmd.Flags().GetInt32("threshold")

				var party []string
				for _, servicePeer := range servicePeerList {
					if len(party) < int(count) {
						party = append(party, servicePeer)
					}
				}

				if len(party) < int(count) {
					logger.Fatal(errors.New("not enough service nodes available to complete the request"))
				}

				keygenArgs := pb.KeygenArgs{
					Count:     count,
					Threshold: threshold,
					Alg:       alg,
					Party:     party,
				}

				data, err := proto.Marshal(&keygenArgs)
				if err != nil {
					logger.Fatal(err)
				}

				if err = client.Publish(ctx, topic, data); err != nil {
					logger.Fatal(err)
				}
			}

			client.RunUntilCancel()
		},
	}

	cmd.Flags().StringP("topic", "T", "", "topic to publish on")
	cmd.Flags().StringP("message", "m", "", "message to signed (in hex)")
	cmd.Flags().StringP("algorithm", "a", "ecdsa", "algorithm the key should work with")
	cmd.Flags().Int32P("count", "c", 0, "total number of peers that should receive a share of the secret")
	cmd.Flags().Int32P("threshold", "t", 0, "number of peers that need to collaborate to use the secret")

	cmd.MarkFlagsMutuallyExclusive("message", "algorithm")
	cmd.MarkFlagsMutuallyExclusive("message", "count")
	cmd.MarkFlagsMutuallyExclusive("message", "threshold")

	cmd.MarkFlagsRequiredTogether("algorithm", "count", "threshold")

	if err := cmd.MarkFlagRequired("topic"); err != nil {
		logger.Fatal(err)
	}

	return cmd
}
