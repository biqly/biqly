package app

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
)

// ResolvedDatasource holds a datasource record, its driver, and an open pool.
type ResolvedDatasource struct {
	Record *metadata.Datasource
	Driver datasource.Driver
	DB     *sql.DB
}

// ResolveDatasourceDB loads the datasource, resolves the driver, decrypts the DSN, and opens a pool.
// The caller must close DB when finished.
func (d *Dependencies) ResolveDatasourceDB(ctx context.Context, id string) (*ResolvedDatasource, error) {
	ds, err := d.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDatasource, err)
	}
	driver, err := d.DriverReg.Get(ds.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDriver, err)
	}
	dsn, err := metadata.RuntimeDSN(ds, d.Encryptor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDatasource, err)
	}
	db, err := driver.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrConnection, err)
	}
	return &ResolvedDatasource{Record: ds, Driver: driver, DB: db}, nil
}
