package cmd

import (
	"github.com/spf13/cobra"
)

// startCmd is used to spin up a node. The type of node being created (bootstrap/peer/client) is indicated as a subcommand.
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a new node",
		Long:  `Creates a new node`,
	}

	cmd.AddCommand(startPeerCmd())
	cmd.AddCommand(startBootstrapCmd())
	cmd.AddCommand(startClientCmd())

	return cmd
}
