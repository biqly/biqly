package forbidden_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Fixture for testpolicy static analysis; not executed as part of normal test-go
// because this directory is excluded from repo walk except in testpolicy self-tests.

func TestRequiresLiveNATS(t *testing.T) {
	q, err := ConnectNATS(NATSConfig{URL: "nats://localhost:4222"})
	require.NoError(t, err)
	_ = q
}

type NATSConfig struct{ URL string }

func ConnectNATS(NATSConfig) (any, error) { return nil, nil }
