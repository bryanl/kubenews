package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd is the root command for kubenews.
var RootCmd = &cobra.Command{
	Use:   "kubenews",
	Short: "Kubenews generates summaries for the Kubernetes project",
}

func init() {
	viper.SetEnvPrefix("KUBENEWS")
	RootCmd.PersistentFlags().String("github_token", "", "Github Token")
	viper.BindPFlag("github_token", RootCmd.PersistentFlags().Lookup("github_token"))
	viper.BindEnv("github_token")
}
