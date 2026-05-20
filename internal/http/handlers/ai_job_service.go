package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
	Request         aiQueryRequest  `json:"request"`
}

func (s *AIJobService) Enqueue(ctx context.Context, kind, sessionID string, req aiQueryRequest) (*metadata.AIJob, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("client_session_id is required")
	}
	if kind != "query" && kind != "preview" && kind != "run" {
		return nil, fmt.Errorf("invalid kind")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	job := &metadata.AIJob{
		ID:              uuid.NewString(),
		ClientSessionID: sessionID,
		Kind:            kind,
		Status:          metadata.AIJobStatusQueued,
		Phase:           "queued",
		PhaseMessage:    "waiting in queue",
		ProgressPct:     0,
		RequestJSON:     raw,
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

func (s *AIJobService) Process(ctx context.Context, jobID string) error {
	if err := s.repo.MarkAIJobRunning(ctx, jobID); err != nil {
		return err
	}
	job, err := s.repo.GetAIJob(ctx, jobID)
	if err != nil {
		return err
	}
	phase, err := aiJobPhaseFromKind(job.Kind)
	if err != nil {
		_ = s.repo.FailAIJob(ctx, jobID, err.Error())
		return err
	}
	var req aiQueryRequest
	if err := json.Unmarshal(job.RequestJSON, &req); err != nil {
		_ = s.repo.FailAIJob(ctx, jobID, "invalid request payload")
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
	resp, err := s.ai.executeAIQueryPhase(ctx, req, phase, report)
	if err != nil {
		_ = s.repo.FailAIJob(ctx, jobID, err.Error())
		return err
	}
	raw, err := encodeAIJobResult(resp)
	if err != nil {
		_ = s.repo.FailAIJob(ctx, jobID, err.Error())
		return err
	}
	return s.repo.CompleteAIJob(ctx, jobID, raw)
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
