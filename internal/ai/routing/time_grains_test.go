package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/stretchr/testify/require"
)

type fakeTimeGrainRepo struct {
	countCalls  int
	listCalls   int
	upsertCalls int

	countVal int
	countErr error

	listVal []metadata.TimeGrain
	listErr error

	upsertErr error
}

func (f *fakeTimeGrainRepo) CountTimeGrains(_ context.Context) (int, error) {
	f.countCalls++
	return f.countVal, f.countErr
}

func (f *fakeTimeGrainRepo) ListTimeGrains(_ context.Context) ([]metadata.TimeGrain, error) {
	f.listCalls++
	return f.listVal, f.listErr
}

func (f *fakeTimeGrainRepo) UpsertTimeGrain(_ context.Context, _ metadata.TimeGrain) error {
	f.upsertCalls++
	return f.upsertErr
}

func TestStaticTimeGrainStore(t *testing.T) {
	store := NewStaticTimeGrainStore()
	grains, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultTimeGrains, grains)
}

func TestDBTimeGrainStore_FallbackToDefaultWhenError(t *testing.T) {
	repo := &fakeTimeGrainRepo{
		listErr: errors.New("database connection error"),
	}
	store := NewDBTimeGrainStore(repo)

	grains, err := store.List(context.Background())
	require.NoError(t, err) // Should fallback gracefully
	require.Equal(t, DefaultTimeGrains, grains)
	require.Equal(t, 1, repo.listCalls)
}

func TestDBTimeGrainStore_FallbackToDefaultWhenEmpty(t *testing.T) {
	repo := &fakeTimeGrainRepo{
		listVal: []metadata.TimeGrain{},
	}
	store := NewDBTimeGrainStore(repo)

	grains, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultTimeGrains, grains)
	require.Equal(t, 1, repo.listCalls)
}

func TestDBTimeGrainStore_CachingAndInvalidation(t *testing.T) {
	customGrains := []metadata.TimeGrain{
		{
			Grain:        "year",
			Suffix:       "_y",
			RequiresTime: false,
			Synonyms:     []string{"yıl"},
		},
	}
	repo := &fakeTimeGrainRepo{
		listVal: customGrains,
	}
	store := NewDBTimeGrainStore(repo)

	// First list: reads from DB
	g1, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, customGrains, g1)
	require.Equal(t, 1, repo.listCalls)

	// Second list: should use cache
	g2, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, customGrains, g2)
	require.Equal(t, 1, repo.listCalls)

	// Invalidate cache
	store.Invalidate()

	// Third list: reads from DB again
	g3, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, customGrains, g3)
	require.Equal(t, 2, repo.listCalls)
}

func TestSeedTimeGrains(t *testing.T) {
	t.Run("Already seeded", func(t *testing.T) {
		repo := &fakeTimeGrainRepo{
			countVal: 5,
		}
		err := SeedTimeGrains(context.Background(), repo)
		require.NoError(t, err)
		require.Equal(t, 1, repo.countCalls)
		require.Equal(t, 0, repo.upsertCalls)
	})

	t.Run("Empty DB - performs seeding", func(t *testing.T) {
		repo := &fakeTimeGrainRepo{
			countVal: 0,
		}
		err := SeedTimeGrains(context.Background(), repo)
		require.NoError(t, err)
		require.Equal(t, 1, repo.countCalls)
		require.Equal(t, len(DefaultTimeGrains), repo.upsertCalls)
	})
}
