package cmd

import (
	"context"
	"errors"
	"github.com/milquellc/codis/network"
	"github.com/milquellc/codis/proto/pb"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	rpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
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

			rpcClient, err := client.StartRPCClient(ctx, cfg.Host)
			if err != nil {
				logger.Fatal(err)
			} else {
				logger.Debug("started RPC client")
			}

			logger.Info("client is running! listening at %s", client.ListenAddrs())

			hostInfo, err := peer.AddrInfoFromString(cfg.Host)
			if err != nil {
				logger.Fatal(err)
			}

			go clientLoop(client, rpcClient, hostInfo.ID)
			client.RunUntilCancel()
		},
	}

	return cmd
}

const (
	signAction   = "sign"
	keygenAction = "keygen"
)

// clientLoop defines the interface for the Codis client. The loop waits for input on all the args needed to run the
// different protocols.
func clientLoop(client *network.Peer, rpcClient *rpc.Client, hostId peer.ID) {
	for {
		// the start prompt is used so that we have a clean idle state, we can read the current peer list at the moment
		// we are ready, and so that the logs for different requests have a point of separation.
		var start string
		startPrompt := &survey.Input{Message: "Your Codis Client is live, press enter to see the menu..."}
		if err := survey.AskOne(startPrompt, &start); err != nil {
			logger.Error(err)
			continue
		}

		// ideally, this list would only consist of service peers on the network so that a user can't start a stream
		// with a node that doesn't support the protocol. however, there is no apparent way to distinguish a bootstrap/
		// service/client node from each other without implementing a custom identify protocol.
		var servicePeerList []string
		for _, p := range client.Host.Peerstore().Peers() {
			if p.String() == client.Host.ID().String() { // don't include self in peer list
				continue
			}
			servicePeerList = append(servicePeerList, p.String())
		}

		startResponse := menuPrompt(hostId, servicePeerList)

		var response interface{}
		switch startResponse.Action {
		case keygenAction:
			protocolArgs := pb.KeygenArgs{
				Party:     startResponse.Party,
				Alg:       startResponse.Alg,
				Count:     int32(len(startResponse.Party)),
				Threshold: int32(startResponse.Threshold),
			}
			protocolReply := pb.KeygenReply{}

			if err := rpcClient.Call(hostId, "KeygenService", "Keygen", &protocolArgs, &protocolReply); err != nil {
				logger.Error(err)
			}
		case signAction:
			response = signPrompt()

			protocolArgs := pb.SignArgs{
				Party:     startResponse.Party,
				Alg:       startResponse.Alg,
				Count:     int32(len(startResponse.Party)),
				Threshold: int32(startResponse.Threshold),
				Message:   response.(signAnswers).Message,
			}
			protocolReply := pb.SignReply{}

			if err := rpcClient.Call(hostId, "SignService", "Sign", &protocolArgs, &protocolReply); err != nil {
				logger.Error(err)
			}
		default:
			logger.Error(errors.New("invalid action"))
		}
	}
}

// startAnswers defines the responses that are necessary for any further operation (in other words, the common parameters
// for each protocol that the client can invoke on its host)
type startAnswers struct {
	Action    string
	Alg       string
	Party     []string
	Threshold int
}

// menuPrompt fills out a startAnswers to form the basis of a client's request to its host node
func menuPrompt(clientHost peer.ID, servicePeerList []string) startAnswers {
	qs := []*survey.Question{
		{
			Name:   "action",
			Prompt: &survey.Select{Message: "Which protocol do you want to run:", Options: []string{"sign", "keygen"}},
		},
		{
			Name: "alg",
			Prompt: &survey.Select{
				Message: "Select DSA for signing/generating compatible key:",
				Options: []string{"ecdsa", "eddsa"},
			},
		},
		{
			Name: "party",
			Prompt: &survey.MultiSelect{
				Message: "Select the participating nodes:",
				Options: servicePeerList,
				Default: []string{clientHost.String()},
			},
		},
		{
			Name: "threshold",
			Prompt: &survey.Input{
				Message: "How many peers need to collaborate to use the secret:",
			},
		},
	}

	var answers startAnswers
	if err := survey.Ask(qs, &answers); err != nil {
		if err == terminal.InterruptErr {
			os.Exit(0)
		}
		logger.Error(err)
	}

	return answers
}

// signAnswers represents the info that supplements startAnswers to form a complete signature request for a service node
type signAnswers struct {
	Message string
}

// signPrompt fills out a signAnswers that is used as part of a client's request to its host to generate a signature
func signPrompt() signAnswers {
	qs := []*survey.Question{
		{
			Name: "message",
			Prompt: &survey.Input{
				Message: "Input the message to be signed (in hex):",
			},
		},
	}

	var answers signAnswers
	if err := survey.Ask(qs, &answers); err != nil {
		if err == terminal.InterruptErr {
			os.Exit(0)
		}
		logger.Error(err)
	}

	return answers
}
