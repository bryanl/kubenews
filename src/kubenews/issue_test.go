package kubenews

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

type anyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

type validJSON struct{}

func (j validJSON) Match(v driver.Value) bool {
	var out []interface{}
	err := json.Unmarshal(v.([]uint8), &out)
	return err == nil
}

func TestImportIssues(t *testing.T) {
	stdlibdb, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := sqlx.NewDb(stdlibdb, "mockdriver")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").WithArgs(1, "open", "title", "body", "user", validJSON{},
		"assignee", anyTime{}, anyTime{}, anyTime{}, "milestone", "org/repo").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO issues").WithArgs(2, "open", "title", "body", "user", validJSON{},
		"assignee", anyTime{}, anyTime{}, anyTime{}, "milestone", "org/repo").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	now := time.Now()

	issue1 := github.Issue{
		Number: github.Int(1),
		State:  github.String("open"),
		Title:  github.String("title"),
		Body:   github.String("body"),
		User:   &github.User{Name: github.String("user")},
		Labels: []github.Label{
			{URL: github.String("http://example.com"), Name: github.String("label1"), Color: github.String("#fff")},
		},
		Assignee:   &github.User{Name: github.String("assignee")},
		ClosedAt:   &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
		Milestone:  &github.Milestone{Title: github.String("milestone")},
		Repository: &github.Repository{FullName: github.String("org/repo")},
	}
	issue2 := github.Issue{
		Number: github.Int(2),
		State:  github.String("open"),
		Title:  github.String("title"),
		Body:   github.String("body"),
		User:   &github.User{Name: github.String("user")},
		Labels: []github.Label{
			{URL: github.String("http://example.com"), Name: github.String("label1"), Color: github.String("#fff")},
		},
		Assignee:   &github.User{Name: github.String("assignee")},
		ClosedAt:   &now,
		CreatedAt:  &now,
		UpdatedAt:  &now,
		Milestone:  &github.Milestone{Title: github.String("milestone")},
		Repository: &github.Repository{FullName: github.String("org/repo")},
	}

	issues := []github.Issue{issue1, issue2}

	err = ImportIssues(db, issues)
	require.NoError(t, err)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}
