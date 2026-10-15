package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muhammadjihad/spock/internal/config"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("spock v%s\n", config.Version)
		},
	})
}
