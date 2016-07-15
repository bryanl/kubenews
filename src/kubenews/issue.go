package kubenews

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
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
	Assignee   string     `db:"assignee"`
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

// Value is an implementation of driver.Value for Labels.
func (l Labels) Value() (driver.Value, error) {
	return json.Marshal(l)
}

// Scan converts a DB value back into Labels.
func (l *Labels) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	b := json.RawMessage(src.([]uint8))
	return json.Unmarshal(b, l)
}

// LastUpdate is the time of issues where last updated
type LastUpdate struct {
	Repository string     `db:"repository"`
	At         *time.Time `db:"updated_at"`
}

// LastIssueUpdate retrieves the last time an issue was updated for a repository.
func LastIssueUpdate(db *sqlx.DB, repository string) (*LastUpdate, error) {
	lastUpdate := LastUpdate{}
	if err := db.Get(&lastUpdate, lastUpdateSQL, repository); err != nil {
		if err == sql.ErrNoRows {
			return &LastUpdate{Repository: repository}, nil
		}

		return nil, errors.Wrap(err, "unable to retrieve last update")
	}

	return &lastUpdate, nil
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

	log.Info("updating or importing issues")
	for _, in := range inIssues {
		issue := ConvertIssue(repository, in)

		if _, err := tx.Exec(insertIssueSQL, issue.Number, issue.State, issue.Title, issue.Body,
			issue.User, issue.Labels, issue.Assignee, issue.ClosedAt, issue.CreatedAt,
			issue.UpdatedAt, issue.Milestone, issue.Repository); err != nil {
			return err
		}
	}

	log.Info("analyzing labels")
	labels := map[string]Label{}

	issue := Issue{}
	rows, err := db.Queryx(activeIssuesSQL)
	if err != nil {
		return errors.Wrap(err, "query open issues failure")
	}
	for rows.Next() {
		err := rows.StructScan(&issue)
		if err != nil {
			return errors.Wrap(err, "scan issue into struct failure")
		}

		for _, label := range issue.Labels {
			labels[label.Name] = label
		}
	}

	for _, label := range labels {
		if _, err := tx.Exec(insertLabelSQL, label.Name, label.URL, label.Color); err != nil {
			return errors.Wrap(err, "insert label")
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
  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)

  ON conflict (number)
  DO UPDATE SET (state, title, body, labels, assignee, closed_at, updated_at, milestone) =
    ($2, $3, $4, $6, $7, $8, $10, $11)
  WHERE issues.number = $1`

	lastUpdateSQL = `
  SELECT updated_at FROM issues
  WHERE repository = $1
  ORDER BY updated_at desc limit 1`

	insertLabelSQL = `
  INSERT INTO labels
  (name, url, color, active)

  VALUES
  ($1, $2, $3, true)

  ON conflict(name)
  DO UPDATE SET (url, color, active) = ($2, $3, true)
  WHERE labels.name = $1`

	activeIssuesSQL = `
  SELECT id, number, state, title, body, created_by, labels, assignee, closed_at,
    created_at, updated_at, milestone, repository
  FROM issues
  WHERE state = 'open'`
)
