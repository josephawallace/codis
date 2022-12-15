package cmd

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/milquellc/codis/network"
	"github.com/milquellc/codis/proto/pb"
	"github.com/milquellc/codis/protocols"
	"github.com/milquellc/codis/utils"
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

			if err := client.AdvertiseConnect(ctx, cfg.Rendezvous); err != nil {
				logger.Error(err)
			} else {
				logger.Debug("peer advertised itself at the %s rendezvous point", cfg.Rendezvous)
			}

			logger.Info("client is running! listening at %s", client.ListenAddrs())

			time.Sleep(time.Second * 5)

			var servicePeerList [][]byte
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
				servicePeerList = append(servicePeerList, []byte(peerId))
			}

			topic, _ := cmd.Flags().GetString("topic")

			switch topic {
			case protocols.KeygenTopic:
				alg, _ := cmd.Flags().GetString("algorithm")
				count, _ := cmd.Flags().GetInt32("count")
				threshold, _ := cmd.Flags().GetInt32("threshold")

				var party [][]byte
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
					Algorithm: []byte(alg),
					Party:     party,
				}
				keygenReply := pb.KeygenReply{}

				peerIds, err := utils.PeerIdsBytesToPeerIds(party)
				if err != nil {
					logger.Fatal(err)
				}
				for _, peerId := range peerIds {
					if err = callKeygen(ctx, client, peerId, protocols.KeygenPId, &keygenArgs, &keygenReply); err != nil {
						logger.Error(err)
						return
					}
				}
			case protocols.SignTopic:
				keyStr, _ := cmd.Flags().GetString("key")
				key, err := hex.DecodeString(keyStr)
				if err != nil {
					logger.Fatal(err)
				}
				messageStr, _ := cmd.Flags().GetString("message")
				message, err := hex.DecodeString(messageStr)
				if err != nil {
					logger.Fatal(err)
				}

				signArgs := pb.SignArgs{
					PublicKey: key,
					Message:   message,
				}

				data, err := proto.Marshal(&signArgs)
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
	cmd.Flags().StringP("key", "k", "", "public key associated with the private key that should sign")
	cmd.Flags().StringP("message", "m", "", "message to signed (in hex)")
	cmd.Flags().StringP("algorithm", "a", "ecdsa", "algorithm the key should work with")
	cmd.Flags().Int32P("count", "c", 0, "total number of peers that should receive a share of the secret")
	cmd.Flags().Int32P("threshold", "t", 0, "number of peers that need to collaborate to use the secret")

	cmd.MarkFlagsMutuallyExclusive("message", "algorithm")
	cmd.MarkFlagsMutuallyExclusive("message", "count")
	cmd.MarkFlagsMutuallyExclusive("message", "threshold")
	cmd.MarkFlagsMutuallyExclusive("key", "algorithm")
	cmd.MarkFlagsMutuallyExclusive("key", "count")
	cmd.MarkFlagsMutuallyExclusive("key", "threshold")

	cmd.MarkFlagsRequiredTogether("algorithm", "count", "threshold")

	if err := cmd.MarkFlagRequired("topic"); err != nil {
		logger.Fatal(err)
	}

	return cmd
}

func callKeygen(ctx context.Context, client *network.Peer, peer peer.ID, pid protocol.ID, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	s, err := client.Host.NewStream(ctx, peer, pid)
	if err != nil {
		return err
	}
	defer s.Close()

	data, err := proto.Marshal(args)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(s)
	if _, err = writer.Write(data); err != nil {
		_ = s.Reset()
		return err
	}
	if err := writer.Flush(); err != nil {
		_ = s.Reset()
		return err
	}

	//go func(s libnetwork.Stream, data []byte, reply *pb.KeygenReply) {
	//	data, err = io.ReadAll(s)
	//	if err != nil {
	//		logger.Fatal(err)
	//	}
	//	err = proto.Unmarshal(data, reply)
	//	if err != nil {
	//		logger.Fatal(err)
	//	}
	//}(s, data, reply)

	return nil
}
