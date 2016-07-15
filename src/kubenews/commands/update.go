package commands

import (
	"kubenews"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	RootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update kubernetes issues",
	Long:  "Retrieve new kubernetes issues and update local data store",
	Run: func(cmd *cobra.Command, args []string) {
		githubToken := viper.GetString("github_token")

		db, err := kubenews.NewDB()
		if err != nil {
			log.WithError(err).Fatal("unable to connect to database")
		}

		gh := kubenews.NewGithub(githubToken)
		repo := "kubernetes/kubernetes"

		lastUpdate, err := kubenews.LastIssueUpdate(db, repo)
		if err != nil {
			log.WithError(err).Fatal("unable to retrieve last issue update")
		}
		log.WithFields(log.Fields{
			"lastUpdate": lastUpdate.At,
			"repo":       repo}).Info("issues last update")

		issues, err := gh.ListRepoIssues(repo, lastUpdate.At)
		if err != nil {
			log.WithError(err).Fatal("list all issues")
		}

		log.WithField("issueCount", len(issues)).Info("triaging issues")

		if err := kubenews.ImportIssues(db, repo, issues); err != nil {
			log.WithError(err).Fatal("cannot import issues")
		}

	},
}
