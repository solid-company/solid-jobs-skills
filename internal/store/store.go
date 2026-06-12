// Package store is the SQLite persistence layer for solid-jobs-skills. All SQL
// lives in query.sql and is compiled by sqlc into the sqlcgen subpackage; this
// package wraps those generated methods behind a small typed API. Business
// logic lives in internal/jobs, which uses this package.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DefaultProfileName is the profile seeded on first run.
const DefaultProfileName = "default"

// Store wraps the SQLite database handle and the sqlc-generated queries.
type Store struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// Open opens (creating if needed) the SQLite database at path, applies the
// schema, and seeds a default profile. Parent directories are created.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	// Pragmas via DSN: enable FK enforcement and a busy timeout.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite single-writer; avoids "database is locked"

	// sqlc only generates query code; the schema is created here at runtime.
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	s := &Store{db: db, q: sqlcgen.New(db)}
	if err := s.seedDefaultProfile(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying handle for advanced callers.
func (s *Store) DB() *sql.DB { return s.db }

// Queries exposes the generated query set (used by the stats package).
func (s *Store) Queries() *sqlcgen.Queries { return s.q }

// bg is the background context used for the synchronous store API.
func bg() context.Context { return context.Background() }

// now returns the current UTC time in RFC3339 for consistent text storage.
func now() string { return time.Now().UTC().Format(time.RFC3339) }

// seedDefaultProfile inserts the default profile if no profiles exist yet.
func (s *Store) seedDefaultProfile() error {
	n, err := s.q.CountProfiles(bg())
	if err != nil {
		return fmt.Errorf("count profiles: %w", err)
	}
	if n > 0 {
		return nil
	}
	ts := now()
	if err := s.q.SeedDefaultProfile(bg(), sqlcgen.SeedDefaultProfileParams{
		Name: DefaultProfileName, CreatedAt: ts, UpdatedAt: ts,
	}); err != nil {
		return fmt.Errorf("seed default profile: %w", err)
	}
	return nil
}

// --- small nullable conversion helpers shared across the store ---

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func strOrEmpty(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
