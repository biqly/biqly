// Package main is the agentic query runner service: a NATS-driven
// planner/tool-execution pipeline that can supersede the legacy
// single-shot NL-to-SQL path once graduated out of shadow mode (see
// internal/http/handlers/ai_job_service.go's routeAIJob).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	observability.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracing, tracErr := observability.SetupTracing(context.Background(), "agent")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	shutdownLogExport, logExpErr := observability.SetupLogExport(ctx, "agent")
	if logExpErr != nil {
		slog.Warn("log export setup failed, continuing with stdout only", "error", logExpErr)
	}
	defer func() {
		if err := shutdownLogExport(context.Background()); err != nil {
			slog.Warn("log provider shutdown error", "error", err)
		}
	}()

	deps, err := app.NewAgentDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize agent dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close agent dependencies", "error", err)
		}
	}()

	httpSrv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      agent.NewServer(deps),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	go func() {
		slog.Info("agent internal server started", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("agent internal server stopped", "error", err)
			os.Exit(1)
		}
	}()

	consumerGroup := cfg.NATS.ConsumerGroup
	if consumerGroup == "" {
		consumerGroup = "biqly-agent-workers"
	}
	go func() {
		err := deps.Queue.Subscribe(ctx, cfg.Agent.JobSubject, consumerGroup, func(jobCtx context.Context, payload []byte) error {
			return processJob(jobCtx, deps, payload)
		})
		if err != nil && ctx.Err() == nil {
			slog.Error("agent job consumer stopped", "error", err)
			os.Exit(1)
		}
	}()
	deps.Ready.Store(true)
	slog.Info("agent started", "nats_url", cfg.NATS.URL, "job_subject", cfg.Agent.JobSubject, "group", consumerGroup)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	deps.Ready.Store(false)
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("agent internal server shutdown error", "error", err)
	}
	slog.Info("agent shutting down")
}

// processJob decodes and validates one agent-job.v1 message, then resumes
// (or starts) its run. Job-id-keyed creation makes this redelivery-safe: a
// message redelivered after a crash resumes the same run instead of
// duplicating it.
func processJob(ctx context.Context, deps *agent.AgentDependencies, payload []byte) error {
	var job agent.Job
	if err := sonic.Unmarshal(payload, &job); err != nil {
		return errors.New("decode agent job: " + err.Error())
	}
	if err := agent.ValidateJob(job); err != nil {
		return err
	}

	run, ok, err := deps.MetaRepo.GetAgentRunByJobID(ctx, job.JobID)
	if err != nil {
		return err
	}
	if !ok {
		runID, err := deps.MetaRepo.CreateAgentRunForJob(ctx, job.JobID, metadata.AgentRunInsert{
			ConversationID: job.ConversationID,
			DatasourceID:   job.DatasourceID,
			ModelID:        job.ModelID,
			UserID:         job.UserID,
			Question:       job.Question,
			QuestionHash:   metadata.QuestionHash(job.Question),
			Mode:           job.Mode,
		})
		if err != nil {
			return err
		}
		run = metadata.AgentRunRow{ID: runID}
	} else {
		// A run already exists for this job_id: job_id-keyed creation
		// (see the doc comment above) makes this only reachable via a NATS
		// redelivery or a crash-recovery retry of the same message.
		deps.Metrics.RecordAgentQueueRedelivery()
	}

	runCtx, runCancel := context.WithCancel(ctx)
	deps.Runs.Register(run.ID, runCancel)
	defer func() {
		runCancel()
		deps.Runs.Unregister(run.ID)
	}()

	runContext := agent.RunContext{
		// This deployment's tenant boundary is the workspace: WorkspaceID
		// authorizes exactly one tenant's data, matching the isolation the
		// rest of the platform already enforces per-workspace.
		TenantID:     job.WorkspaceID,
		UserID:       job.UserID,
		DatasourceID: job.DatasourceID,
		Question:     job.Question,
		AllowedTools: []agent.ToolName{
			agent.ToolCatalog, agent.ToolSemantic, agent.ToolQueryCompile, agent.ToolQueryExecute, agent.ToolMemoryRecall,
		},
		// ExternalEgressTools is intentionally empty: every tool here calls an
		// in-cluster BI service over HTTP, not a third-party API directly.
		// Airgapped-mode LLM/embedding egress is enforced inside the AI
		// service itself (providerpkg.SetAirgapped), not re-checked here.
		DeploymentMode:         deps.Config.DeploymentMode,
		MaxRows:                job.MaxRows,
		Timeout:                time.Duration(job.TimeoutSeconds) * time.Second,
		MaxSteps:               job.MaxSteps,
		MaxClarificationRounds: job.MaxClarificationRounds,
		// HiddenColumns/PIIColumns/AllowedJoins/RequiredRowFilter are not yet
		// populated from the resolved semantic model — see docs/superpowers
		// for the follow-up that wires per-datasource column/join policy into
		// RunContext. Until then, the query tools' own compile-time
		// validation (via the real Query service) is the enforcement point
		// for those checks.
	}

	_, err = deps.Runtime.Run(runCtx, runContext, run.ID)
	return err
}
