package postgres

import (
	"context"
	"database/sql"
	"errors"

	// Import the PostgreSQL driver.
	"github.com/lib/pq"
	pkgerrors "github.com/pkg/errors"

	"github.com/usememos/memos/internal/dbutil"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

type DB struct {
	db      *sql.DB
	profile *profile.Profile
	// Add any other fields as needed
}

func NewDB(profile *profile.Profile) (store.Driver, error) {
	if profile == nil {
		return nil, errors.New("profile is nil")
	}

	// Open the PostgreSQL connection
	db, err := sql.Open("postgres", profile.DSN)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to open database: %s", profile.DSN)
	}
	dbutil.ApplySQLConnectionSettings(db, "postgres")
	if err := dbutil.Ping(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, pkgerrors.Wrap(err, "failed to ping postgres")
	}

	var driver store.Driver = &DB{
		db:      db,
		profile: profile,
	}

	// Return the DB struct
	return driver, nil
}

func (d *DB) GetDB() *sql.DB {
	return d.db
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) EnsureColumns(ctx context.Context) error {
	return ensurePostgresMemoPayload(ctx, d.db)
}

func ensurePostgresMemoPayload(ctx context.Context, db *sql.DB) error {
	var tableExists int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'memo' LIMIT 1").Scan(&tableExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return pkgerrors.Wrap(err, "failed to inspect memo table")
	}
	const checkColumn = `
SELECT 1
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'memo'
  AND column_name = 'payload'
LIMIT 1;
`
	var columnExists int
	if err := db.QueryRowContext(ctx, checkColumn).Scan(&columnExists); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return pkgerrors.Wrap(err, "failed to inspect memo table")
	}

	if _, err := db.ExecContext(ctx, "ALTER TABLE memo ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'"); err != nil {
		// Idempotent guard: tolerate duplicate-column race between concurrent starters.
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "42701" {
			return nil
		}
		return pkgerrors.Wrap(err, "failed to add payload column to memo table")
	}
	return nil
}
