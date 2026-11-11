package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pkg/errors"

	// Import the SQLite driver.
	_ "modernc.org/sqlite"

	"github.com/usememos/memos/internal/dbutil"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

type DB struct {
	db      store.DBTX
	pool    *sql.DB
	profile *profile.Profile
}

// NewDB opens a database specified by its database driver name and a
// driver-specific data source name, usually consisting of at least a
// database name and connection information.
func NewDB(profile *profile.Profile) (store.Driver, error) {
	// Ensure a DSN is set before attempting to open the database.
	if profile.DSN == "" {
		return nil, errors.New("dsn required")
	}

	// Connect to the database with some sane settings:
	// - No shared-cache: it's obsolete; WAL journal mode is a better solution.
	// - No foreign key constraints: it's currently disabled by default, but it's a
	// good practice to be explicit and prevent future surprises on SQLite upgrades.
	// - Journal mode set to WAL: it's the recommended journal mode for most applications
	// as it prevents locking issues.
	//
	// Notes:
	// - When using the `modernc.org/sqlite` driver, each pragma must be prefixed with `_pragma=`.
	//
	// References:
	// - https://pkg.go.dev/modernc.org/sqlite#Driver.Open
	// - https://www.sqlite.org/sharedcache.html
	// - https://www.sqlite.org/pragma.html
	sqliteDB, err := sql.Open("sqlite", profile.DSN+"?_pragma=foreign_keys(0)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-2048)&_pragma=temp_store(FILE)&_pragma=mmap_size(0)")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db with dsn: %s", profile.DSN)
	}
	dbutil.ApplySQLConnectionSettings(sqliteDB, "sqlite")
	if err := dbutil.Ping(context.Background(), sqliteDB); err != nil {
		_ = sqliteDB.Close()
		return nil, errors.Wrap(err, "failed to ping sqlite")
	}

	driver := DB{db: sqliteDB, profile: profile}

	return &driver, nil
}

func (d *DB) GetDB() *sql.DB {
	if d.pool != nil {
		return d.pool
	}
	return d.db.(*sql.DB)
}

func (d *DB) Close() error {
	return d.GetDB().Close()
}

func (d *DB) EnsureColumns(ctx context.Context) error {
	return ensureSQLiteMemoPayload(ctx, d.GetDB())
}

func ensureSQLiteMemoPayload(ctx context.Context, db *sql.DB) error {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type='table' AND name='memo' LIMIT 1").Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return errors.Wrap(err, "failed to inspect memo table")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(memo)")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return errors.Wrap(err, "failed to inspect memo table")
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return errors.Wrap(err, "failed to scan table info")
		}
		if name == "payload" {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "failed to read table info")
	}

	if found {
		return nil
	}

	if _, err := db.ExecContext(ctx, "ALTER TABLE memo ADD COLUMN `payload` TEXT NOT NULL DEFAULT '{}';"); err != nil {
		// Idempotent guard: tolerate duplicate-column race between concurrent starters.
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return nil
		}
		return errors.Wrap(err, "failed to add payload column to memo table")
	}
	return nil
}

// WithTx returns a driver bound to tx without changing the shared driver.
func (d *DB) WithTx(tx *sql.Tx) store.Driver {
	scoped := *d
	scoped.pool = d.GetDB()
	scoped.db = tx
	return &scoped
}
