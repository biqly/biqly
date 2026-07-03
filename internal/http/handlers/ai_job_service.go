package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/queue"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type AIJobService struct {
	repo      *metadata.Repository
	publisher queue.AIJobPublisher
	ai        *AIHandler
}

func NewAIJobService(repo *metadata.Repository, publisher queue.AIJobPublisher, aiHandler *AIHandler) *AIJobService {
	return &AIJobService{repo: repo, publisher: publisher, ai: aiHandler}
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
	datasourceID, scopeSchemas, err := s.enqueueJobScope(ctx, kind, req)
	if err != nil {
		return nil, err
	}
	var userIDPtr *string
	if strings.TrimSpace(userID) != "" {
		userIDPtr = new(userID)
	}
	jobLocale := aiJobLocaleFromRequest(kind, req, string(i18n.FromContext(ctx)))
	job := &metadata.AIJob{
		ID:              uuid.NewString(),
		ClientSessionID: sessionID,
		UserID:          userIDPtr,
		Locale:          jobLocale,
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
			if failErr := s.repo.FailAIJob(ctx, job.ID, err.Error()); failErr != nil {
				slog.WarnContext(ctx, "mark async AI job failed", "job_id", job.ID, "err", failErr)
			}
			return nil, err
		}
	}
	return job, nil
}

func aiJobLocaleFromRequest(kind string, raw json.RawMessage, fallback string) string {
	locale := strings.TrimSpace(fallback)
	switch kind {
	case "describe":
		var req ai.DescribeRequest
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err == nil && strings.TrimSpace(req.Locale) != "" {
			locale = strings.TrimSpace(req.Locale)
		}
	case "describe_batch":
		var req ai.DescribeBatchRequest
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err == nil && strings.TrimSpace(req.Locale) != "" {
			locale = strings.TrimSpace(req.Locale)
		}
	}
	return string(i18n.ParseLocale(locale))
}

func (s *AIJobService) enqueueJobScope(ctx context.Context, kind string, req json.RawMessage) (*string, []string, error) {
	switch kind {
	case "describe_batch":
		return s.describeBatchEnqueueScope(ctx, req)
	case "embed_metadata":
		return s.embedMetadataEnqueueScope(ctx, req)
	default:
		return nil, []string{}, nil
	}
}

func (s *AIJobService) describeBatchEnqueueScope(ctx context.Context, req json.RawMessage) (*string, []string, error) {
	var batchReq ai.DescribeBatchRequest
	if err := sonic.ConfigStd.Unmarshal(req, &batchReq); err != nil {
		return nil, nil, errors.New("invalid request payload")
	}
	ds := strings.TrimSpace(batchReq.DatasourceID)
	if ds == "" {
		return nil, nil, errors.New("datasource_id is required")
	}
	scopeSchemas := ai.DescribeBatchScopeSchemas(batchReq.Tables)
	if len(scopeSchemas) == 0 {
		return nil, nil, errors.New("tables must include at least one schema")
	}
	existing, err := s.repo.FindConflictingDescribeBatch(ctx, ds, scopeSchemas)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, &AIJobConflictError{
			Message:    "metadata describe batch already running for overlapping schema(s)",
			ExistingID: existing.ID,
			Existing:   existing,
		}
	}
	return new(ds), scopeSchemas, nil
}

func (s *AIJobService) embedMetadataEnqueueScope(ctx context.Context, req json.RawMessage) (*string, []string, error) {
	var er embedMetadataRequest
	if err := sonic.ConfigStd.Unmarshal(req, &er); err != nil {
		return nil, nil, errors.New("invalid request payload")
	}
	ds := strings.TrimSpace(er.DatasourceID)
	if ds == "" {
		return nil, nil, errors.New(core.MsgDatasourceIDRequired)
	}
	model := strings.TrimSpace(er.ModelID)
	existing, err := s.repo.FindConflictingEmbedMetadata(ctx, ds, model)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, &AIJobConflictError{
			Message:    "embedding refresh already running for the same scope",
			ExistingID: existing.ID,
			Existing:   existing,
		}
	}
	return new(ds), []string{}, nil
}

func validateAIJobRequest(kind string, raw json.RawMessage) error {
	switch kind {
	case "query", "preview", "run":
		var req aiQueryRequest
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err != nil {
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
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err != nil {
			return errors.New("invalid request payload")
		}
		if req.DatasourceID == "" || req.Table == "" {
			return errors.New("datasource_id and table are required")
		}
	case "describe_batch":
		var req ai.DescribeBatchRequest
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err != nil {
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
		if err := sonic.ConfigStd.Unmarshal(raw, &req); err != nil {
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
	// Re-attach request-scoped values stripped by the async worker's bare consumer
	// context: user identity for per-user model preferences, locale for i18n strings.
	ctx = consumerContextForAIJob(ctx, job)
	raw, err := s.processJob(ctx, job, report)
	if err != nil {
		// The failure-bookkeeping reads/writes must not ride the job context: on
		// worker shutdown that context is already cancelled, so reusing it would
		// silently fail and leave the job stuck in "running" forever.
		bgCtx := context.WithoutCancel(ctx)
		job, getErr := s.repo.GetAIJob(bgCtx, jobID)
		if getErr == nil && job.Status == metadata.AIJobStatusCancelled {
			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "cancelled") {
			return nil
		}
		// A cancelled/expired job context means worker shutdown or deadline, not a
		// genuine job failure — return the error so the message redelivers and the
		// job can resume, without recording a spurious permanent failure.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		failCtx, cancel := context.WithTimeout(bgCtx, 10*time.Second)
		defer cancel()
		if failErr := s.repo.FailAIJob(failCtx, jobID, err.Error()); failErr != nil {
			slog.WarnContext(bgCtx, "mark async AI job failed", "job_id", jobID, "err", failErr)
		}
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
		if err := sonic.ConfigStd.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		resp, err := s.ai.executeAIQueryPhase(ctx, req, phase, report)
		if err != nil {
			return nil, err
		}
		return encodeAIJobResult(resp)
	case "describe":
		var req ai.DescribeRequest
		if err := sonic.ConfigStd.Unmarshal(job.RequestJSON, &req); err != nil {
			return nil, errors.New("invalid request payload")
		}
		result, err := s.ai.executeMetadataDescribeJob(ctx, req, report)
		if err != nil {
			return nil, err
		}
		return encodeDescribeJobResult(result)
	case "describe_batch":
		var req ai.DescribeBatchRequest
		if err := sonic.ConfigStd.Unmarshal(job.RequestJSON, &req); err != nil {
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
		if err := sonic.ConfigStd.Unmarshal(job.RequestJSON, &req); err != nil {
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

func consumerContextForAIJob(ctx context.Context, job *metadata.AIJob) context.Context {
	if job == nil {
		return ctx
	}
	if job.UserID != nil && *job.UserID != "" {
		ctx = ai.WithUserID(ctx, *job.UserID)
	}
	if loc := strings.TrimSpace(job.Locale); loc != "" {
		ctx = i18n.WithLocale(ctx, i18n.ParseLocale(loc))
	}
	return ctx
}

type dynamicSemaphore struct {
	mu      sync.Mutex
	active  int
	waiting int
	ch      chan struct{}
	limit   func() int
}

func newDynamicSemaphore(limitFn func() int) *dynamicSemaphore {
	return &dynamicSemaphore{
		ch:    make(chan struct{}, 1024),
		limit: limitFn,
	}
}

func (s *dynamicSemaphore) Acquire(ctx context.Context) error {
	for {
		s.mu.Lock()
		lim := s.limit()
		if lim <= 0 {
			lim = 1
		}
		if s.active < lim {
			s.active++
			s.mu.Unlock()
			return nil
		}
		s.waiting++
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.waiting--
			s.mu.Unlock()
			return ctx.Err()
		case <-s.ch:
			s.mu.Lock()
			s.waiting--
			s.mu.Unlock()
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *dynamicSemaphore) Release() {
	s.mu.Lock()
	if s.active > 0 {
		s.active--
	}
	lim := s.limit()
	if lim <= 0 {
		lim = 1
	}
	toWake := 0
	if s.waiting > 0 && s.active < lim {
		toWake = min(lim-s.active, s.waiting)
	}
	s.mu.Unlock()
	for range toWake {
		select {
		case s.ch <- struct{}{}:
		default:
		}
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

	sem := newDynamicSemaphore(func() int {
		if s.ai == nil {
			return 1
		}
		return s.ai.EffectiveConcurrency(ctx)
	})

	return consumer.Subscribe(ctx, group, func(cctx context.Context, jobID string) error {
		if err := sem.Acquire(cctx); err != nil {
			return err
		}
		defer sem.Release()
		return s.Process(cctx, jobID)
	})
}

func encodeEmbedMetadataJobResult(result *embedMetadataResponse) (json.RawMessage, error) {
	if result == nil {
		return nil, nil
	}
	return sonic.ConfigStd.Marshal(result)
}
