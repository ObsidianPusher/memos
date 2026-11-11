package db

import (
	"github.com/pkg/errors"

	profilepkg "github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db/mysql"
	"github.com/usememos/memos/store/db/postgres"
	"github.com/usememos/memos/store/db/sqlite"
)

// NewDBDriver creates new db driver based on profile.
func NewDBDriver(profile *profilepkg.Profile) (store.Driver, error) {
	var driver store.Driver
	var err error

	switch profile.Driver {
	case profilepkg.DriverSQLite:
		driver, err = sqlite.NewDB(profile)
	case profilepkg.DriverMySQL:
		driver, err = mysql.NewDB(profile)
	case profilepkg.DriverPostgres:
		driver, err = postgres.NewDB(profile)
	default:
		return nil, errors.New("unknown db driver")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to create db driver")
	}
	return driver, nil
}
