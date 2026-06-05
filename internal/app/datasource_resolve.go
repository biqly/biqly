package app

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
)

// ResolvedDatasource holds a datasource record, its driver, and an open pool.
type ResolvedDatasource struct {
	Record *metadata.Datasource
	Driver datasource.Driver
	DB     *sql.DB
}

// ResolveDatasourceDB loads the datasource for CatalogDeps.
func (d *CatalogDeps) ResolveDatasourceDB(ctx context.Context, id string) (*ResolvedDatasource, error) {
	return resolveDatasourceDBHelper(ctx, d.MetaRepo, d.DriverReg, d.Encryptor, id)
}

// ResolveDatasourceDB loads the datasource for AIDeps.
func (d *AIDeps) ResolveDatasourceDB(ctx context.Context, id string) (*ResolvedDatasource, error) {
	return resolveDatasourceDBHelper(ctx, d.MetaRepo, d.DriverReg, d.Encryptor, id)
}

func resolveDatasourceDBHelper(ctx context.Context, metaRepo *metadata.Repository, driverReg *datasource.Registry, encryptor *security.Encryption, id string) (*ResolvedDatasource, error) {
	ds, err := metaRepo.GetDatasource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDatasource, err)
	}
	driver, err := driverReg.Get(ds.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDriver, err)
	}
	dsn, err := metadata.RuntimeDSN(ds, encryptor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrLoadDatasource, err)
	}
	db, err := driver.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrConnection, err)
	}
	return &ResolvedDatasource{Record: ds, Driver: driver, DB: db}, nil
}
