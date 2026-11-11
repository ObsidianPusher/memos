package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	pkgerrors "github.com/pkg/errors"

	"github.com/usememos/memos/internal/dbutil"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

type DB struct {
	db      store.DBTX
	pool    *sql.DB
	profile *profile.Profile
	config  *mysql.Config
}

func NewDB(profile *profile.Profile) (store.Driver, error) {
	// Open MySQL connection with parameter.
	// multiStatements=true is required for migration.
	// See more in: https://github.com/go-sql-driver/mysql#multistatements
	dsn, err := mergeDSN(profile.DSN)
	if err != nil {
		return nil, err
	}

	driver := DB{profile: profile}
	driver.config, err = mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("Parse DSN eroor")
	}

	driver.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to open db: %s", profile.DSN)
	}
	dbutil.ApplySQLConnectionSettings(driver.GetDB(), "mysql")
	if err := dbutil.Ping(context.Background(), driver.GetDB()); err != nil {
		_ = driver.GetDB().Close()
		return nil, pkgerrors.Wrap(err, "failed to ping mysql")
	}

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
	return ensureMySQLMemoPayload(ctx, d.GetDB())
}

func ensureMySQLMemoPayload(ctx context.Context, db *sql.DB) error {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'memo' LIMIT 1").Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return pkgerrors.Wrap(err, "failed to inspect memo table")
	}
	const checkColumn = `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = 'memo'
  AND column_name = 'payload'
LIMIT 1;
`
	var column string
	if err := db.QueryRowContext(ctx, checkColumn).Scan(&column); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return pkgerrors.Wrap(err, "failed to inspect memo table")
	}

	if _, err := db.ExecContext(ctx, "ALTER TABLE `memo` ADD COLUMN `payload` JSON NOT NULL DEFAULT '{}';"); err != nil {
		return pkgerrors.Wrap(err, "failed to add payload column to memo table")
	}
	return nil
}

func mergeDSN(baseDSN string) (string, error) {
	config, err := mysql.ParseDSN(baseDSN)
	if err != nil {
		return "", pkgerrors.Wrapf(err, "failed to parse DSN: %s", baseDSN)
	}

	config.MultiStatements = true
	return config.FormatDSN(), nil
}

// WithTx returns a driver bound to tx without changing the shared driver.
func (d *DB) WithTx(tx *sql.Tx) store.Driver {
	scoped := *d
	scoped.pool = d.GetDB()
	scoped.db = tx
	return &scoped
}
