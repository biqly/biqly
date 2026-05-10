// Package app wires together application dependencies.
package app

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/clickhouse"
	"github.com/biqly/biqly/internal/datasource/mysql"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/datasource/sqlserver"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
)

// Dependencies holds all application dependencies.
type Dependencies struct {
	Config         *config.Config
	MetadataDB     *sql.DB
	DriverReg      *datasource.Registry
	MetaRepo       *metadata.Repository
	SemanticRepo   *semantic.Repository
	Validator      *query.Validator
	Executor       *query.Executor
	AIClient       ai.Provider
	AIDescriber    *ai.DescribeService
}

// NewDependencies wires up all dependencies.
func NewDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	// Connect to metadata database
	db, err := sql.Open("pgx", cfg.Metadata.DSN)
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping metadata db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Setup driver registry
	reg := datasource.NewRegistry()
	reg.Register(postgres.NewDriver())
	reg.Register(mysql.NewDriver())
	reg.Register(sqlserver.NewDriver())
	reg.Register(clickhouse.NewDriver())

	// Setup repositories
	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)

	// Setup query components
	validator := query.NewValidator(cfg.Query.MaxRows)
	executor := query.NewExecutor(cfg.Query.MaxRows, cfg.QueryTimeout())

	// AI provider (OpenAI / Anthropic / OpenAI-compatible) + metadata describe
	// service. Failing here is fatal so misconfigured deployments stop early
	// instead of crashing on first request.
	aiClient, err := ai.NewProvider(cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("ai provider: %w", err)
	}
	describer := ai.NewDescribeService(aiClient, metaRepo, reg, 10, cfg.AI.DescribeMaxCellRunes, cfg.AI.DescribeMaxSampleRows)

	return &Dependencies{
		Config:       cfg,
		MetadataDB:   db,
		DriverReg:    reg,
		MetaRepo:     metaRepo,
		SemanticRepo: semanticRepo,
		Validator:    validator,
		Executor:     executor,
		AIClient:     aiClient,
		AIDescriber:  describer,
	}, nil
}

// Close cleans up resources.
func (d *Dependencies) Close() error {
	if d.MetadataDB != nil {
		return d.MetadataDB.Close()
	}
	return nil
}
