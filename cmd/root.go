package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/muhammadjihad/spock/internal/config"
	"github.com/muhammadjihad/spock/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "spock",
	Short: "SSH key, connection, and session manager",
	Long: `Spock is a human-friendly CLI for managing SSH keys, connections, and sessions.

Examples:
  spock ssh                          List saved connections
  spock ssh to worker-1              Connect to worker-1
  spock ssh add staging              Add a new connection
  spock key                          List managed keys
  spock key create --name deploy     Generate a new SSH key
  spock session                      List active sessions
  spock session mirror calm-tiger    Watch an active session`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.EnsureDirs()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
