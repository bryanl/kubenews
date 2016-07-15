package kubenews

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// Issue is a Github issue.
type Issue struct {
	ID         int        `db:"id"`
	Number     int        `db:"number"`
	State      string     `db:"state"`
	Title      string     `db:"title"`
	Body       string     `db:"body"`
	User       string     `db:"created_by"`
	Labels     Labels     `db:"labels"`
	Assignee   string     `db:"asignee"`
	ClosedAt   *time.Time `db:"closed_at"`
	CreatedAt  *time.Time `db:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"`
	Milestone  string     `db:"milestone"`
	Repository string     `db:"repository"`
}

// Label is a Github label.
type Label struct {
	URL   string
	Name  string
	Color string
}

// Labels is a slice of Label.
type Labels []Label

// Value is an implementation of driver.Value for Label.
func (l Labels) Value() (driver.Value, error) {
	return json.Marshal(l)
}

// ImportIssues imports issues to our datastore. If the issue exists, it is updated.
func ImportIssues(db *sqlx.DB, repository string, inIssues []github.Issue) error {
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "import issue failure")
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}

		err = tx.Commit()
	}()

	for _, in := range inIssues {
		issue := ConvertIssue(repository, in)

		if _, err := tx.Exec(insertIssueSQL, issue.Number, issue.State, issue.Title, issue.Body,
			issue.User, issue.Labels, issue.Assignee, issue.ClosedAt, issue.CreatedAt,
			issue.UpdatedAt, issue.Milestone, issue.Repository); err != nil {
			return err
		}
	}

	return nil
}

// ConvertIssue converts an issue from the github api client to our format.
func ConvertIssue(repostitory string, in github.Issue) Issue {

	issue := Issue{
		Number:     *in.Number,
		State:      *in.State,
		Title:      *in.Title,
		Labels:     []Label{},
		ClosedAt:   in.ClosedAt,
		CreatedAt:  in.CreatedAt,
		UpdatedAt:  in.UpdatedAt,
		Repository: repostitory,
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, in)
			panic("unknown data")
		}
	}()

	if in.Body != nil {
		issue.Body = *in.Body
	}

	if in.User != nil {
		issue.User = *in.User.Login
	}

	if in.Assignee != nil {
		issue.Assignee = *in.Assignee.Login
	}

	if in.Milestone != nil {
		issue.Milestone = *in.Milestone.Title
	}

	for _, ghLabel := range in.Labels {
		issue.Labels = append(issue.Labels, ConvertLabel(ghLabel))
	}

	return issue
}

// ConvertLabel converts a label from the github api client to our format.
func ConvertLabel(in github.Label) Label {
	return Label{
		URL:   *in.URL,
		Name:  *in.Name,
		Color: *in.Color,
	}
}

var (
	insertIssueSQL = `
  INSERT INTO issues
  (number, state, title, body, created_by, labels, assignee, closed_at, created_at,
  updated_at, milestone, repository)

  VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
)
