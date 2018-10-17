package cmd

import (
	"fmt"

	"github.com/c-base/cion/config"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Config())
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
