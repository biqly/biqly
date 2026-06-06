package handlers

import (
	"github.com/bytedance/sonic"
	"net/http"

	"github.com/biqly/biqly/internal/metadata"
)

type AIJobConflictError struct {
	Message    string
	ExistingID string
	Existing   *metadata.AIJob
}

func (e *AIJobConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "describe batch already running for overlapping schemas"
}

type aiJobConflictResponse struct {
	Error        string          `json:"error"`
	ExistingID   string          `json:"existing_job_id"`
	ExistingJob  *metadata.AIJob `json:"existing_job,omitempty"`
	ScopeSchemas []string        `json:"scope_schemas,omitempty"`
}

func writeAIJobConflict(w http.ResponseWriter, err *AIJobConflictError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	body := aiJobConflictResponse{
		Error:       err.Error(),
		ExistingID:  err.ExistingID,
		ExistingJob: err.Existing,
	}
	if err.Existing != nil {
		body.ScopeSchemas = err.Existing.ScopeSchemas
	}
	if err := sonic.ConfigStd.NewEncoder(w).Encode(body); err != nil {
		return
	}
}
