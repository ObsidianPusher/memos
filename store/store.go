package store

import (
	"context"

	"github.com/usememos/memos/internal/profile"
)

// Store provides database access to all raw objects.
type Store struct {
	profile *profile.Profile
	driver  Driver

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
