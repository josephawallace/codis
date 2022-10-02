package cmd

import (
	"codis/pkg/p2p"
	"codis/pkg/utils"
	"encoding/hex"
	"github.com/spf13/cobra"
	"log"
)

// startPeerCmd starts a "regular" node. This node reaches out to the given bootstrap nodes to learn about the network.
// After discovering the peers on the network, the node can participate in the cryptographic protocols.
func startPeerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Starts a new peer",
		Long:  `Creates a new peer that connects to bootstrap nodes.`,
		Run: func(c *cobra.Command, args []string) {
			bootstrapAddrs := utils.StringsToAddrs(config.BootstrapAddrs)
			psk, err := hex.DecodeString(config.PSK)
			if err != nil {
				log.Fatalf("Failed to decode PSK")
			}
			// Again, the peer is identical to the bootstrap node. The only difference being that this peer will connect
			// to the peers on the network by announcing itself at the rendezvous point at the given bootstrap nodes. This
			// means the peer can act as a bootstrap node if it chooses.
			p2phost := p2p.NewP2P(bootstrapAddrs, config.Rendezvous, psk)
			p2phost.AdvertiseConnect()
			p2phost.StartRPCServer()
			p2phost.RunUntilCancel()
		},
	}

	// We differentiate a bootstrap node and a peer by whether it has any knowledge of a network outside itself. If the
	// node is to be considered a peer, we must give a bootstrap address as the entrypoint to larger network.
	cmd.PersistentFlags().StringSliceVar(&config.BootstrapAddrs, "bootstrap", []string{}, "bootstrap addrs")
	if err := cmd.MarkPersistentFlagRequired("bootstrap"); err != nil {
		log.Fatalf("Failed to set flag as required: %+v", err)
	}

	return cmd
}
