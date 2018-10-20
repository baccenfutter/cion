package cmd

import (
	"github.com/baccenfutter/cion/api"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start API backend and serve all requests.",
	Run: func(cmd *cobra.Command, args []string) {
		api.ListenAndServe()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
