package dbutil

import (
	"context"
	"database/sql"
	"runtime"
	"time"
)

const (
	defaultPingTimeout     = 5 * time.Second
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultConnMaxLifetime = 30 * time.Minute
	defaultMaxIdleConns    = 5
	defaultMinimumMaxConns = 10
)

func ApplySQLConnectionSettings(db *sql.DB, driver string) {
	if db == nil {
		return
	}

	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxIdleTime(0)
		db.SetConnMaxLifetime(0)
		return
	}

	maxOpenConns := runtime.GOMAXPROCS(0) * 4
	if maxOpenConns < defaultMinimumMaxConns {
		maxOpenConns = defaultMinimumMaxConns
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
}

func Ping(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	return db.PingContext(pingCtx)
}
