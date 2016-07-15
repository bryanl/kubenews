package kubenews

import (
	"github.com/jmoiron/sqlx"

	// we're using postgres
	_ "github.com/lib/pq"
)

// NewDB logs into the database and returns an error if it fails.
func NewDB() (*sqlx.DB, error) {
	return sqlx.Connect("postgres", "user=postgres dbname=kubenews sslmode=disable")
}
