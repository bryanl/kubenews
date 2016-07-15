package kubenews

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

var (
	// workerCount is the amount of workers to load for simultaneous retrieval of issues.
	workerCount = 3

	// perPageCount is the items per page when listing from the github api.
	perPageCount = 100

	// githubRateLimit is the maximum amount of calls to make during a time period.
	githubRateLimit = time.Second / 3

	// minThrottleDelay is the amount of time to wait when the github api throttles.
	minThrottleDelay = time.Second * 30
)

// Github is a Github client.
type Github struct {
	client *github.Client
}

// NewGithub creates an instance of Github.
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

func splitRepo(repoName string) (string, string, error) {
	repoParts := strings.Split(repoName, "/")
	if len(repoParts) != 2 {
		return "", "", errors.Errorf("invalid repo name %s", repoName)
	}

	org := repoParts[0]
	repo := repoParts[1]

	return org, repo, nil
}

// ListRepoIssues lists issues for a repository. It returns a list of issues and
// an error if an exception occurs.
func (gh *Github) ListRepoIssues(repoName string) ([]github.Issue, error) {
	org, repo, err := splitRepo(repoName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	allIssues, resp, err := gh.GetRepoIssuePage(org, repo, 0)
	if err != nil {
		return nil, err
	}

	pageChan := make(chan int, 100)

	outChan := make(chan github.Issue, 100)
	go func() {
		for issue := range outChan {
			allIssues = append(allIssues, issue)
		}
	}()

	wg := sync.WaitGroup{}

	// start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(ctx context.Context, workerID int) {
			w := worker{
				id:   workerID,
				gh:   gh,
				org:  org,
				repo: repo,
			}

			defer wg.Done()
			if err := w.start(ctx, pageChan, outChan); err != nil {
				log.WithError(err).Error("worker failed")
				cancel()
			}
		}(ctx, i)
	}

	log.WithField("totalPages", resp.LastPage).Info("page info")

	// throttle the requests, so we don't anger the github api
	throttle := time.Tick(githubRateLimit)

	for i := 1; i <= resp.LastPage; i++ {
		<-throttle
		pageChan <- i
	}
	close(pageChan)

	log.Info("waiting for workers to finish")
	wg.Wait()

	return allIssues, nil
}

// GetRepoIssuePage retrieves issues by page.
func (gh *Github) GetRepoIssuePage(org, repo string, page int) ([]github.Issue, *github.Response, error) {
	logger := log.WithField("currentPage", page)
	logger.Debug("fetching page")
	issueOptions := &github.IssueListByRepoOptions{
		State: "all",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPageCount,
		},
	}

	issues, resp, err := gh.client.Issues.ListByRepo(org, repo, issueOptions)
	if err != nil {
		if resp.StatusCode == 403 {
			buffer := time.Duration(rand.Int31n(30)) * time.Second
			delayTime := minThrottleDelay + buffer
			logger.WithField("delayTime", delayTime).Warn("github api throttled, delaying")
			time.Sleep(delayTime)
			return gh.GetRepoIssuePage(org, repo, page)
		}

		logger.WithError(err).Error("listing page")
		return nil, nil, errors.Wrap(err, "issue retrieval failed")
	}

	logger.WithFields(log.Fields{
		"lastPage": resp.LastPage,
		"apiCalls": resp.Rate.Remaining}).Info("fetched page")

	newIssues := []github.Issue{}
	for _, issue := range issues {
		newIssues = append(newIssues, *issue)
	}

	return newIssues, resp, err
}

type worker struct {
	id   int
	gh   *Github
	org  string
	repo string
}

func (w *worker) start(ctx context.Context, pageChan chan int, out chan github.Issue) error {
	logger := log.WithFields(log.Fields{"workerID": w.id})
	logger.Info("starting up")
	done := false
	for !done {
		select {
		case <-ctx.Done():
			logger.Info("worker canceled")
			return nil
		case page, ok := <-pageChan:
			if !ok {
				logger.Info("closed channel detected: shutting down")
				done = true
			} else {
				issues, _, err := w.gh.GetRepoIssuePage(w.org, w.repo, page)
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
