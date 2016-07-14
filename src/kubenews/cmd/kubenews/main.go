package main

import (
	"fmt"
	"sort"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/oauth2"
)

type specification struct {
	GithubToken string `envconfig:"github_token" required:"true"`
}

func main() {
	var s specification
	if err := envconfig.Process("kubenews", &s); err != nil {
		log.WithError(err).Fatal("unable to read configuration from env")
	}

	gh := NewGithub(s.GithubToken)

	issueMap := map[int]github.Issue{}
	labels := map[string][]github.Issue{}

	issues, err := gh.ListRepoIssues("kubernetes", "kubernetes")
	if err != nil {
		log.WithError(err).Fatal("list all issues")
	}

	for _, issue := range issues {
		issueMap[*issue.ID] = *issue
		fmt.Println(*issue.Title)

		for _, label := range issue.Labels {
			name := *label.Name
			if labels[name] == nil {
				labels[name] = []github.Issue{}
			}

			labels[name] = append(labels[name], *issue)
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

type Github struct {
	client *github.Client
}

func NewGithub(token string) *Github {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	client.UserAgent = "kubenews"

	gh := &Github{
		client: client,
	}

	return gh
}

func (gh *Github) ListRepoIssues(org, repo string) ([]*github.Issue, error) {
	allIssues, resp, err := gh.ListRepoIssue(org, repo, 1)
	if err != nil {
		return nil, err
	}

	workerCount := 5

	pageChan := make(chan int, 100)

	outChan := make(chan *github.Issue, 100)
	go func() {
		for issue := range outChan {
			allIssues = append(allIssues, issue)
		}
	}()

	wg := sync.WaitGroup{}

	// start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			w := worker{
				id:   workerID,
				gh:   gh,
				org:  "kubernetes",
				repo: "kubernetes",
			}

			if err := w.start(pageChan, outChan); err != nil {
				log.WithError(err).Error("worker")
			}
			wg.Done()
		}(i)
	}

	log.WithField("totalPages", resp.LastPage).Info("page info")
	for i := 2; i <= resp.LastPage; i++ {
		pageChan <- i
	}
	close(pageChan)

	log.Info("waiting for workers to finish")
	wg.Wait()

	return allIssues, nil
}

func (gh *Github) ListRepoIssue(org, repo string, page int) ([]*github.Issue, *github.Response, error) {
	logger := log.WithField("currentPage", page)
	logger.Info("fetching page")
	issueOptions := &github.IssueListByRepoOptions{
		State: "all",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: 100,
		},
	}

	issues, resp, err := gh.client.Issues.ListByRepo(org, repo, issueOptions)
	if err != nil {
		logger.WithError(err).Error("listing page")
	}

	logger.WithFields(log.Fields{
		"lastPage": resp.LastPage,
		"apiCalls": resp.Rate.Remaining}).Info("fetched page")

	return issues, resp, err
}

type worker struct {
	id   int
	gh   *Github
	org  string
	repo string
}

func (w *worker) start(pageChan chan int, out chan *github.Issue) error {
	logger := log.WithFields(log.Fields{"workerID": w.id})
	logger.Info("starting up")
	done := false
	for !done {
		select {
		case page, ok := <-pageChan:
			if !ok {
				logger.Info("closed channel detected")
				done = true
			} else {
				issues, _, err := w.gh.ListRepoIssue(w.org, w.repo, page)
				if err != nil {
					return err
				}

				for _, issue := range issues {
					out <- issue
				}
			}
		}
	}

	return nil
}
