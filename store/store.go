package store

import (
	"context"
	"log/slog"

	"github.com/usememos/memos/internal/profile"
)

// Store provides database access to all raw objects.
type Store struct {
	profile       *profile.Profile
	driver        Driver
	inTransaction bool
	afterCommit   []func(*Store) error

	// No in-memory cache (explicitly disabled).
}

// New creates a new instance of Store.
func New(driver Driver, profile *profile.Profile) *Store {
	store := &Store{
		driver:  driver,
		profile: profile,
	}

	return store
}

func (s *Store) GetDriver() Driver {
	return s.driver
}

func (s *Store) Close() error {
	return s.driver.Close()
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	return s.driver.EnsureColumns(ctx)
}

// WithTransaction commits only after all database work succeeds. Nested calls
// share the existing transaction; filesystem and external effects stay outside.
func (s *Store) WithTransaction(ctx context.Context, fn func(*Store) error) error {
	if s.inTransaction {
		return fn(s)
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	scoped := &Store{profile: s.profile, driver: s.driver.WithTx(tx), inTransaction: true}
	if err := fn(scoped); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, cleanup := range scoped.afterCommit {
		if err := cleanup(s); err != nil {
			slog.Warn("failed to clean up resource after commit", slog.Any("err", err))
		}
	}
	return nil
}
