package routing

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/biqly/biqly/internal/metadata"
)

// TimeGrainStore defines the interface for fetching customizable time grains.
type TimeGrainStore interface {
	List(ctx context.Context) ([]metadata.TimeGrain, error)
	Invalidate()
}

// DefaultTimeGrains is the hardcoded compile-time defaults.
var DefaultTimeGrains = []metadata.TimeGrain{
	{
		Grain:        "year",
		Suffix:       "_year",
		RequiresTime: false,
		Synonyms:     []string{"year", "years", "yearly", "annual", "yıl", "yil", "yıllık", "yillik", "per year", "by year"},
	},
	{
		Grain:        "quarter",
		Suffix:       "_quarter",
		RequiresTime: false,
		Synonyms:     []string{"quarter", "quarters", "qtr", "çeyrek", "ceyrek", "çeyreklik", "ceyreklik"},
	},
	{
		Grain:        "month",
		Suffix:       "_month",
		RequiresTime: false,
		Synonyms:     []string{"month", "months", "monthly", "ay", "aylık", "aylik", "per month", "by month"},
	},
	{
		Grain:        "day",
		Suffix:       "_day",
		RequiresTime: false,
		Synonyms:     []string{"day", "days", "daily", "gün", "gun", "günlük", "gunluk", "per day", "by day", "günü", "gunu"},
	},
	{
		Grain:        "hour",
		Suffix:       "_hour",
		RequiresTime: true,
		Synonyms:     []string{"hour", "hours", "hourly", "saat", "saatlik", "saatte", "saatli", "per hour", "by hour"},
	},
}

type staticTimeGrainStore struct{}

func (s staticTimeGrainStore) List(_ context.Context) ([]metadata.TimeGrain, error) {
	return DefaultTimeGrains, nil
}

func (s staticTimeGrainStore) Invalidate() {}

// NewStaticTimeGrainStore returns a store serving compile-time defaults.
func NewStaticTimeGrainStore() TimeGrainStore {
	return staticTimeGrainStore{}
}

type timeGrainRepo interface {
	CountTimeGrains(ctx context.Context) (int, error)
	ListTimeGrains(ctx context.Context) ([]metadata.TimeGrain, error)
	UpsertTimeGrain(ctx context.Context, tg metadata.TimeGrain) error
}

type dbTimeGrainStore struct {
	repo   timeGrainRepo
	mu     sync.RWMutex
	cache  []metadata.TimeGrain
	loaded bool
}

// NewDBTimeGrainStore creates a TimeGrainStore backed by the database.
func NewDBTimeGrainStore(repo timeGrainRepo) TimeGrainStore {
	return &dbTimeGrainStore{
		repo: repo,
	}
}

func (s *dbTimeGrainStore) List(ctx context.Context) ([]metadata.TimeGrain, error) {
	s.mu.RLock()
	if s.loaded {
		res := make([]metadata.TimeGrain, len(s.cache))
		copy(res, s.cache)
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-checked locking
	if s.loaded {
		res := make([]metadata.TimeGrain, len(s.cache))
		copy(res, s.cache)
		return res, nil
	}

	grains, err := s.repo.ListTimeGrains(ctx)
	if err != nil {
		slog.WarnContext(ctx, "failed to list time grains from DB, falling back to static defaults", "error", err)
		return DefaultTimeGrains, nil
	}

	if len(grains) == 0 {
		// Table is empty, fallback to defaults
		return DefaultTimeGrains, nil
	}

	s.cache = grains
	s.loaded = true

	res := make([]metadata.TimeGrain, len(s.cache))
	copy(res, s.cache)
	return res, nil
}

func (s *dbTimeGrainStore) Invalidate() {
	s.mu.Lock()
	s.loaded = false
	s.cache = nil
	s.mu.Unlock()
}

// SeedTimeGrains seeds the default time grains into the database if empty.
func SeedTimeGrains(ctx context.Context, repo timeGrainRepo) error {
	if repo == nil {
		return nil
	}

	count, err := repo.CountTimeGrains(ctx)
	if err != nil {
		return fmt.Errorf("seed time grains count: %w", err)
	}

	if count > 0 {
		return nil
	}

	for _, tg := range DefaultTimeGrains {
		if err := repo.UpsertTimeGrain(ctx, tg); err != nil {
			return fmt.Errorf("seed time grain %s: %w", tg.Grain, err)
		}
	}

	slog.InfoContext(ctx, "seeded ai_time_grains table with default values")
	return nil
}
