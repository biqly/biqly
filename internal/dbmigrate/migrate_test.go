package dbmigrate

import (
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMigrationsDirDot(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	abs, err := ResolveMigrationsDir(".")
	require.NoError(t, err)
	assert.Equal(t, cwd, abs)
}

func TestUpToDownFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		up   string
		want string
	}{
		{name: "paired auth migration", up: "001a_create_users.up.sql", want: "001b_create_users.down.sql"},
		{name: "not up suffix", up: "001a_create_users.sql", want: ""},
		{name: "missing a marker", up: "001_create_users.up.sql", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, upToDownFilename(tt.up))
		})
	}
}

func TestIsAlreadyAppliedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "duplicate table", err: &pgconn.PgError{Code: "42P07"}, want: true},
		{name: "duplicate column", err: &pgconn.PgError{Code: "42701"}, want: true},
		{name: "unrelated postgres error", err: &pgconn.PgError{Code: "23503"}, want: false},
		{name: "non postgres error", err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isAlreadyAppliedError(tt.err))
		})
	}
}
