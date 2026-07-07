package main

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/agent"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessJobRejectsMalformedPayload(t *testing.T) {
	err := processJob(context.Background(), nil, []byte("not json"))
	require.Error(t, err)
}

func TestProcessJobRejectsInvalidJob(t *testing.T) {
	// Well-formed JSON, but fails agent.ValidateJob (missing every required
	// field) — must be rejected before anything touches deps.
	err := processJob(context.Background(), nil, []byte(`{"schema":"agent-job.v1"}`))
	require.Error(t, err)
}

func TestProcessJobRejectsUnsupportedSchemaVersion(t *testing.T) {
	job := map[string]any{
		"schema":        "agent-job.v99",
		"job_id":        "job-1",
		"run_id":        "run-1",
		"datasource_id": "ds-1",
		"user_id":       "user-1",
		"mode":          agent.ModeShadow,
	}
	raw, err := sonic.Marshal(job)
	require.NoError(t, err)

	perr := processJob(context.Background(), nil, raw)
	require.Error(t, perr)
	assert.ErrorIs(t, perr, agent.ErrUnsupportedSchemaVersion)
}
