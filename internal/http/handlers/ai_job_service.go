package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/queue"
	"github.com/google/uuid"
)

type AIJobService struct {
	repo      *metadata.Repository
	publisher queue.AIJobPublisher
	ai        *AIHandler
}

func NewAIJobService(repo *metadata.Repository, publisher queue.AIJobPublisher, ai *AIHandler) *AIJobService {
	return &AIJobService{repo: repo, publisher: publisher, ai: ai}
}

type createAIJobRequest struct {
	ClientSessionID string          `json:"client_session_id"`
	Kind            string          `json:"kind"`
	Request         json.RawMessage `json:"request"`
}

func (s *AIJobService) Enqueue(ctx context.Context, kind, sessionID, userID string, req json.RawMessage) (*metadata.AIJob, error) {
	if sessionID == "" {
		return nil, errors.New("client_session_id is required")
	}
	if err := validateAIJobRequest(kind, req); err != nil {
		return nil, err
	}
	if len(req) == 0 {
		req = []byte("{}")
	}
	var datasourceID *string
	scopeSchemas := []string{}
	if kind == "describe_batch" {
		var batchReq ai.DescribeBatchRequest
		if err := json.Unmarshal(req, &batchReq); err != nil {
			return nil, errors.New("invalid request payload")
		}
		ds := strings.TrimSpace(batchReq.DatasourceID)
		if ds == "" {
			return nil, errors.New("datasource_id is required")
		}
		datasourceID = &ds
		scopeSchemas = ai.DescribeBatchScopeSchemas(batchReq.Tables)
		if len(scopeSchemas) == 0 {
			return nil, errors.New("tables must include at least one schema")
		}
		existing, err := s.repo.FindConflictingDescribeBatch(ctx, ds, scopeSchemas)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, &AIJobConflictError{
				Message:    "metadata describe batch already running for overlapping schema(s)",
				ExistingID: existing.ID,
				Existing:   existing,
			}
		}
	}
	if kind == "embed_metadata" {
		var er embedMetadataRequest
		if err := json.Unmarshal(req, &er); err != nil {
			return nil, errors.New("invalid request payload")
		}
		ds := strings.TrimSpace(er.DatasourceID)
		if ds == "" {
			return nil, errors.New(core.MsgDatasourceIDRequired)
		}
		datasourceID = &ds
		model := strings.TrimSpace(er.ModelID)
		existing, err := s.repo.FindConflictingEmbedMetadata(ctx, ds, model)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, &AIJobConflictError{
				Message:    "embedding refresh already running for the same scope",
				ExistingID: existing.ID,
				Existing:   existing,
			}
		}
	}
	var userIDPtr *string
	if strings.TrimSpace(userID) != "" {
		u := userID
		userIDPtr = &u
	}
	job := &metadata.AIJob{
		ID:              uuid.NewString(),
		ClientSessionID: sessionID,
		UserID:          userIDPtr,
		Kind:            kind,
		Status:          metadata.AIJobStatusQueued,
		Phase:           "queued",
		PhaseMessage:    "waiting in queue",
		ProgressPct:     0,
		DatasourceID:    datasourceID,
		ScopeSchemas:    scopeSchemas,
		RequestJSON:     req,
	}
	if err := s.repo.CreateAIJob(ctx, job); err != nil {
		return nil, err
	}
	if s.publisher != nil {
		if err := s.publisher.Publish(ctx, job.ID); err != nil {
			_ = s.repo.FailAIJob(ctx, job.ID, err.Error())
			return nil, err
		}
	}
	return job, nil
}

func validateAIJobRequest(kind string, raw json.RawMessage) error {
	switch kind {
	case "query", "preview", "run":
		var req aiQueryRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return errors.New("invalid request payload")
		}
		if req.Question == "" {
			return errors.New("question is required")
		}
		if req.DatasourceID == "" {
			return errors.New(core.MsgDatasourceIDRequired)
		}
	case "describe":
		var req ai.DescribeRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return errors.New("invalid request payload")
		}
		if req.DatasourceID == "" || req.Table == "" {
			return errors.New("datasource_id and table are required")
		}
	case "describe_batch":
		var req ai.DescribeBatchRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return errors.New("invalid request payload")
		}
		if req.DatasourceID == "" || len(req.Tables) == 0 {
			return errors.New("datasource_id and tables are required")
		}
		if len(req.Tables) > 200 {
			return errors.New("at most 200 tables per batch")
		}
	case "embed_metadata":
		var req embedMetadataRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return errors.New("invalid request payload")
		}
		if req.DatasourceID == "" {
			return errors.New(core.MsgDatasourceIDRequired)
		}
	default:
		return errors.New("invalid kind")
	}
	return nil
}

func (s *AIJobService) Cancel(ctx context.Context, jobID string) (*metadata.AIJob, error) {
	ok, err := s.repo.CancelAIJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if !ok {
		job, getErr := s.repo.GetAIJob(ctx, jobID)
		if getErr != nil {
			return nil, getErr
		}
		switch job.Status {
		case metadata.AIJobStatusSucceeded, metadata.AIJobStatusFailed, metadata.AIJobStatusCancelled:
			return job, nil
		default:
			return nil, errors.New("job cannot be cancelled")
		}
	}
	return s.repo.GetAIJob(ctx, jobID)
}

func (s *AIJobService) Process(ctx context.Context, jobID string) error {
	started, err := s.repo.TryMarkAIJobRunning(ctx, jobID)
	if err != nil {
		return err
	}
	if !started {
		job, getErr := s.repo.GetAIJob(ctx, jobID)
		if getErr != nil {
			return getErr
		}
		if job.Status == metadata.AIJobStatusCancelled {
			return nil
		}
		return fmt.Errorf("job %s is not runnable (status=%s)", jobID, job.Status)
	}
	job, err := s.repo.GetAIJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status == metadata.AIJobStatusCancelled {
		return nil
	}
	report := func(p AIJobProgress) {
		status := p.Status
		if status == "" {
			status = metadata.AIJobStatusRunning
		}
		if uerr := s.repo.UpdateAIJobProgressDetail(ctx, jobID, status, p.Phase, p.Message, p.Progress, p.Detail); uerr != nil {
			slog.WarnContext(ctx, "ai job progress update failed", "job_id", jobID, "error", uerr)
		}
	}
	// Re-attach the submitting user's identity so per-user model preferences
	// (resolved by the AI provider store) apply inside the async worker, which
	// otherwise runs with a bare consumer context.
	if job.UserID != nil && *job.UserID != "" {
		ctx = ai.WithUserID(ctx, *job.UserID)
	}
	raw, err := s.processJob(ctx, job, report)
	if err != nil {
		job, getErr := s.repo.GetAIJob(ctx, jobID)
		if getErr == nil && job.Status == metadata.AIJobStatusCancelled {
			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "cancelled") {
			return nil
		}
		_ = s.repo.FailAIJob(ctx, jobID, err.Error())
		return err
	}
	return s.repo.CompleteAIJob(ctx, jobID, raw)
}

func (s *AIJobService) processJob(ctx context.Context, job *metadata.AIJob, report AIJobProgressFunc) (json.RawMessage, error) {
	switch job.Kind {
	case "query", "preview", "run":
		phase, err := aiJobPhaseFromKind(job.Kind)
		if err != nil {
			return nil, err
		}
		var req aiQueryRequest
		if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		resp, err := s.ai.executeAIQueryPhase(ctx, req, phase, report)
		if err != nil {
			return nil, err
		}
		return encodeAIJobResult(resp)
	case "describe":
		var req ai.DescribeRequest
		if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		result, err := s.ai.executeMetadataDescribeJob(ctx, req, report)
		if err != nil {
			return nil, err
		}
		return encodeDescribeJobResult(result)
	case "describe_batch":
		var req ai.DescribeBatchRequest
		if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		result, err := s.ai.executeMetadataDescribeBatchJob(ctx, job.ID, req, report)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "cancelled") {
				return encodeDescribeBatchJobResult(result)
			}
			return nil, err
		}
		return encodeDescribeBatchJobResult(result)
	case "embed_metadata":
		var req embedMetadataRequest
		if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		result, err := s.ai.executeEmbedMetadataJob(ctx, req, report)
		if err != nil {
			return nil, err
		}
		return encodeEmbedMetadataJobResult(result)
	default:
		return nil, fmt.Errorf("unknown job kind %q", job.Kind)
	}
}

func (s *AIJobService) StartConsumer(ctx context.Context, group string) error {
	if s.publisher == nil {
		return nil
	}
	consumer, ok := s.publisher.(queue.AIJobConsumer)
	if !ok {
		return nil
	}
	return consumer.Subscribe(ctx, group, func(cctx context.Context, jobID string) error {
		return s.Process(cctx, jobID)
	})
}

func encodeEmbedMetadataJobResult(result *embedMetadataResponse) (json.RawMessage, error) {
	if result == nil {
		return nil, nil
	}
	return json.Marshal(result)
}
