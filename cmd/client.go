package cmd

import (
	"codis/p2p"
	"codis/proto/pb"
	"context"
	"github.com/AlecAivazis/survey/v2"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
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
			client := p2p.NewPeer(ctx, cfg.Peers[peerCfgId])

			rpcClient, err := client.StartRPCClient(ctx, cfg.Peers[peerCfgId].Host)
			if err != nil {
				logger.Fatal(err)
			} else {
				logger.Debug("started RPC client")
			}

			logger.Info("client is running! listening at %s", client.ListenAddrs())

			hostAddr, err := multiaddr.NewMultiaddr(cfg.Peers[peerCfgId].Host)
			if err != nil {
				logger.Fatal(err)
			}
			hostInfo, err := peer.AddrInfoFromP2pAddr(hostAddr)
			if err != nil {
				logger.Fatal(err)
			}

			for {
				var start string
				startPrompt := &survey.Input{Message: "Your Codis Client is live, press enter to see the menu..."}
				if err := survey.AskOne(startPrompt, &start); err != nil {
					logger.Fatal(err)
				}

				var action string
				actionPrompt := &survey.Select{Message: "Which action do you want to perform:", Options: []string{"sign", "keygen"}}
				if err := survey.AskOne(actionPrompt, &action); err != nil {
					logger.Fatal(err)
				}

				var partyPeerSelections []string
				for _, p := range client.Host.Peerstore().Peers() {
					if p.String() == hostInfo.ID.String() || p.String() == client.Host.ID().String() {
						continue
					}
					partyPeerSelections = append(partyPeerSelections, p.String())
				}

				switch action {
				case "keygen":
					answers := runKeygenPrompt(partyPeerSelections)

					var alg string
					if answers.Curve == "ed25519" {
						alg = "eddsa"
					} else {
						alg = "ecdsa"
					}

					quorum := strings.Split(answers.Quorum, "/")
					threshold, err := strconv.Atoi(quorum[0])
					if err != nil {
						logger.Error(err)
						return
					}
					count, err := strconv.Atoi(quorum[1])
					if err != nil {
						logger.Error(err)
						return
					}

					answers.Party = append(answers.Party, hostInfo.ID.String())
					protocolArgs := pb.KeygenArgs{Alg: alg, Threshold: int32(threshold), Count: int32(count), Party: answers.Party}
					protocolReply := pb.KeygenReply{}

					if err = rpcClient.Call(hostInfo.ID, "KeygenService", "Keygen", &protocolArgs, &protocolReply); err != nil {
						logger.Error(err)
						return
					}
					break
				case "sign":
					runSign()
					break
				default:
					logger.Info("invalid action")
				}
			}
		},
	}

	return cmd
}

type keygenAnswers struct {
	Curve  string
	Quorum string
	Party  []string
}

func runKeygenPrompt(peerStoreIds []string) keygenAnswers {
	qs := []*survey.Question{
		{
			Name: "curve",
			Prompt: &survey.Select{
				Message: "Select the curve on which to create the key:",
				Options: []string{"secp256k1", "ed25519"},
			},
		},
		{
			Name: "quorum",
			Prompt: &survey.Input{
				Message: "Specify the quorum that will be required to use the secret (threshold/count):",
			},
		},
		{
			Name: "party",
			Prompt: &survey.MultiSelect{
				Message: "Select the other participating clients (your host node is automatically included):",
				Options: peerStoreIds,
			},
		},
	}

	var answers keygenAnswers
	if err := survey.Ask(qs, &answers); err != nil {
		logger.Fatal(err)
	}

	return answers
}

func runSign() {
	logger.Info("Sign needs to be implemented.")
}
