package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

func (s *AIJobService) Enqueue(ctx context.Context, kind, sessionID string, req json.RawMessage) (*metadata.AIJob, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("client_session_id is required")
	}
	if err := validateAIJobRequest(kind, req); err != nil {
		return nil, err
	}
	if len(req) == 0 {
		req = []byte("{}")
	}
	job := &metadata.AIJob{
		ID:              uuid.NewString(),
		ClientSessionID: sessionID,
		Kind:            kind,
		Status:          metadata.AIJobStatusQueued,
		Phase:           "queued",
		PhaseMessage:    "waiting in queue",
		ProgressPct:     0,
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
			return fmt.Errorf("invalid request payload")
		}
		if req.Question == "" {
			return fmt.Errorf("question is required")
		}
		if req.DatasourceID == "" {
			return fmt.Errorf(core.MsgDatasourceIDRequired)
		}
	case "describe":
		var req ai.DescribeRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return fmt.Errorf("invalid request payload")
		}
		if req.DatasourceID == "" || req.Table == "" {
			return fmt.Errorf("datasource_id and table are required")
		}
	default:
		return fmt.Errorf("invalid kind")
	}
	return nil
}

func (s *AIJobService) Process(ctx context.Context, jobID string) error {
	if err := s.repo.MarkAIJobRunning(ctx, jobID); err != nil {
		return err
	}
	job, err := s.repo.GetAIJob(ctx, jobID)
	if err != nil {
		return err
	}
	report := func(p AIJobProgress) {
		status := p.Status
		if status == "" {
			status = metadata.AIJobStatusRunning
		}
		if uerr := s.repo.UpdateAIJobProgress(ctx, jobID, status, p.Phase, p.Message, p.Progress); uerr != nil {
			slog.WarnContext(ctx, "ai job progress update failed", "job_id", jobID, "error", uerr)
		}
	}
	raw, err := s.processJob(ctx, job, report)
	if err != nil {
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
			return nil, fmt.Errorf("invalid request payload")
		}
		resp, err := s.ai.executeAIQueryPhase(ctx, req, phase, report)
		if err != nil {
			return nil, err
		}
		return encodeAIJobResult(resp)
	case "describe":
		var req ai.DescribeRequest
		if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, fmt.Errorf("invalid request payload")
		}
		result, err := s.ai.executeMetadataDescribeJob(ctx, req, report)
		if err != nil {
			return nil, err
		}
		return encodeDescribeJobResult(result)
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
