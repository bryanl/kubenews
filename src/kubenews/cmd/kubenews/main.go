package main

import (
	"fmt"
	"kubenews"
	"sort"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/kelseyhightower/envconfig"
)

type specification struct {
	GithubToken string `envconfig:"github_token" required:"true"`
}

func main() {
	var s specification
	if err := envconfig.Process("kubenews", &s); err != nil {
		log.WithError(err).Fatal("unable to read configuration from env")
	}

	db, err := kubenews.NewDB()
	if err != nil {
		log.WithError(err).Fatal("unable to connect to database")
	}

	gh := kubenews.NewGithub(s.GithubToken)

	issueMap := map[int]github.Issue{}
	labels := map[string][]github.Issue{}

	repo := "kubernetes/kubernetes"

	issues, err := gh.ListRepoIssues(repo)
	if err != nil {
		log.WithError(err).Fatal("list all issues")
	}

	if err := kubenews.ImportIssues(db, repo, issues); err != nil {
		log.WithError(err).Fatal("cannot import issues")
	}

	for _, issue := range issues {
		issueMap[*issue.ID] = issue
		fmt.Println(*issue.Title)

		for _, label := range issue.Labels {
			name := *label.Name
			if labels[name] == nil {
				labels[name] = []github.Issue{}
			}

			labels[name] = append(labels[name], issue)
		}
	}

	var sortedKeys []string
	for k := range labels {
		sortedKeys = append(sortedKeys, k)
	}

	sort.StringSlice(sortedKeys).Sort()

	for _, k := range sortedKeys {
		v := labels[k]
		fmt.Printf("label: %v: %d\n", k, len(v))
	}

	fmt.Printf("%#v\n", issues[len(issues)-1])
}
