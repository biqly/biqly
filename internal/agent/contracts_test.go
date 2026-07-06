package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validJob() Job {
	return Job{
		Schema:                 JobSchemaV1,
		JobID:                  "job-1",
		RunID:                  "run-1",
		DatasourceID:           "ds-1",
		UserID:                 "user-1",
		Mode:                   ModeShadow,
		MaxSteps:               3,
		MaxClarificationRounds: 1,
		TimeoutSeconds:         30,
		MaxRows:                500,
		CreatedAt:              time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
	}
}

func TestValidateJobAcceptsValidEnvelope(t *testing.T) {
	assert.NoError(t, ValidateJob(validJob()))
}

func TestValidateJobRejectsWrongSchemaKind(t *testing.T) {
	j := validJob()
	j.Schema = StepSchemaV1
	err := ValidateJob(j)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnsupportedSchemaVersion), "wrong envelope kind is a shape error, not a version error")
}

func TestValidateJobRejectsUnsupportedMajorVersion(t *testing.T) {
	j := validJob()
	j.Schema = "agent-job.v2"
	err := ValidateJob(j)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSchemaVersion))
}

func TestValidateJobRequiresIDs(t *testing.T) {
	tests := []struct {
		name  string
		clear func(*Job)
	}{
		{"job_id", func(j *Job) { j.JobID = "" }},
		{"run_id", func(j *Job) { j.RunID = "" }},
		{"datasource_id", func(j *Job) { j.DatasourceID = "" }},
		{"user_id", func(j *Job) { j.UserID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := validJob()
			tt.clear(&j)
			assert.Error(t, ValidateJob(j))
		})
	}
}

func TestValidateJobMode(t *testing.T) {
	j := validJob()
	j.Mode = "active"
	assert.NoError(t, ValidateJob(j))

	j.Mode = "eager"
	assert.Error(t, ValidateJob(j))
}

func TestValidateJobMaxSteps(t *testing.T) {
	for _, v := range []int{1, 6} {
		j := validJob()
		j.MaxSteps = v
		assert.NoError(t, ValidateJob(j), "max_steps=%d should be in range", v)
	}
	for _, v := range []int{0, 7} {
		j := validJob()
		j.MaxSteps = v
		assert.Error(t, ValidateJob(j), "max_steps=%d should be out of range", v)
	}
}

func TestValidateJobClarificationRounds(t *testing.T) {
	for _, v := range []int{0, 2} {
		j := validJob()
		j.MaxClarificationRounds = v
		assert.NoError(t, ValidateJob(j), "rounds=%d should be in range", v)
	}
	for _, v := range []int{-1, 3} {
		j := validJob()
		j.MaxClarificationRounds = v
		assert.Error(t, ValidateJob(j), "rounds=%d should be out of range", v)
	}
}

func TestValidateJobTimeoutSeconds(t *testing.T) {
	for _, v := range []int{1, 45} {
		j := validJob()
		j.TimeoutSeconds = v
		assert.NoError(t, ValidateJob(j), "timeout=%d should be in range", v)
	}
	for _, v := range []int{0, 46} {
		j := validJob()
		j.TimeoutSeconds = v
		assert.Error(t, ValidateJob(j), "timeout=%d should be out of range", v)
	}
}

func TestValidateJobMaxRows(t *testing.T) {
	for _, v := range []int{1, 1000} {
		j := validJob()
		j.MaxRows = v
		assert.NoError(t, ValidateJob(j), "max_rows=%d should be in range", v)
	}
	for _, v := range []int{0, 1001} {
		j := validJob()
		j.MaxRows = v
		assert.Error(t, ValidateJob(j), "max_rows=%d should be out of range", v)
	}
}

func validStep() Step {
	return Step{
		Schema:    StepSchemaV1,
		JobID:     "job-1",
		RunID:     "run-1",
		Seq:       1,
		Tool:      "catalog.resolve",
		Status:    "ok",
		CreatedAt: time.Now().UTC(),
	}
}

func TestValidateStepAcceptsValidEnvelope(t *testing.T) {
	assert.NoError(t, ValidateStep(validStep()))
}

func TestValidateStepRejectsUnsupportedVersion(t *testing.T) {
	s := validStep()
	s.Schema = "agent-step.v9"
	err := ValidateStep(s)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSchemaVersion))
}

func TestValidateStepRequiresPositiveSeq(t *testing.T) {
	s := validStep()
	s.Seq = 0
	assert.Error(t, ValidateStep(s))
}

func TestValidateStepRequiresTool(t *testing.T) {
	s := validStep()
	s.Tool = ""
	assert.Error(t, ValidateStep(s))
}

func validResult() Result {
	return Result{
		Schema:     ResultSchemaV1,
		JobID:      "job-1",
		RunID:      "run-1",
		Status:     "completed",
		Confidence: 0.9,
		Answer:     "42",
		RowCount:   1,
		CreatedAt:  time.Now().UTC(),
	}
}

func TestValidateResultAcceptsValidEnvelope(t *testing.T) {
	assert.NoError(t, ValidateResult(validResult()))
}

func TestValidateResultRejectsOutOfRangeConfidence(t *testing.T) {
	r := validResult()
	r.Confidence = 1.5
	assert.Error(t, ValidateResult(r))

	r2 := validResult()
	r2.Confidence = -0.1
	assert.Error(t, ValidateResult(r2))
}

func TestValidateResultRejectsWrongSchema(t *testing.T) {
	r := validResult()
	r.Schema = ErrorSchemaV1
	err := ValidateResult(r)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnsupportedSchemaVersion))
}

func validErrorEnvelope() Error {
	return Error{
		Schema:     ErrorSchemaV1,
		JobID:      "job-1",
		RunID:      "run-1",
		ReasonCode: "policy_denied",
		Message:    "denied",
		CreatedAt:  time.Now().UTC(),
	}
}

func TestValidateErrorAcceptsValidEnvelope(t *testing.T) {
	assert.NoError(t, ValidateError(validErrorEnvelope()))
}

func TestValidateErrorRequiresReasonCode(t *testing.T) {
	e := validErrorEnvelope()
	e.ReasonCode = ""
	assert.Error(t, ValidateError(e))
}

func TestValidateErrorRejectsUnsupportedVersion(t *testing.T) {
	e := validErrorEnvelope()
	e.Schema = "agent-error.v2"
	err := ValidateError(e)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSchemaVersion))
}

func TestIsValidMode(t *testing.T) {
	assert.True(t, IsValidMode(ModeShadow))
	assert.True(t, IsValidMode(ModeActive))
	assert.False(t, IsValidMode("eager"))
	assert.False(t, IsValidMode(""))
}
