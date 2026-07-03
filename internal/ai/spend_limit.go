package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

// ErrSpendLimitExceeded is returned by SpendLimiter.Check when a workspace has
// reached its daily LLM token budget.
var ErrSpendLimitExceeded = errors.New("workspace AI token budget exceeded")

// metricSpendLimitRejections counts requests blocked by the daily budget so an
// alert can flag a workspace hitting its cap (budget set too low, or abuse).
var metricSpendLimitRejections = promauto.NewCounter(prometheus.CounterOpts{
	Name: "biqly_ai_spend_limit_rejections_total",
	Help: "Total AI requests rejected because the workspace reached its daily token budget",
})

// SpendLimiter enforces a per-workspace, per-UTC-day cap on total LLM tokens
// (prompt+completion) to bound provider cost. Counters live in Redis so the cap
// is consistent across all API/AI pods. It fails OPEN on Redis errors: the cap
// is a cost guardrail, not a security control, so a counter-store outage must
// never block legitimate work.
type SpendLimiter struct {
	client      *redis.Client
	dailyBudget int
}

// NewSpendLimiter returns a limiter. A nil client or dailyBudget <= 0 yields a
// disabled limiter whose Check always allows and Record is a no-op.
func NewSpendLimiter(client *redis.Client, dailyBudget int) *SpendLimiter {
	return &SpendLimiter{client: client, dailyBudget: dailyBudget}
}

func (l *SpendLimiter) enabled(workspaceID string) bool {
	return l != nil && l.client != nil && l.dailyBudget > 0 && workspaceID != ""
}

func (*SpendLimiter) key(workspaceID string, now time.Time) string {
	return fmt.Sprintf("ai_spend:%s:%s", workspaceID, now.UTC().Format("20060102"))
}

// Check reports ErrSpendLimitExceeded when the workspace has already met or
// exceeded its daily token budget. Disabled limiter or Redis error → allow.
func (l *SpendLimiter) Check(ctx context.Context, workspaceID string) error {
	if !l.enabled(workspaceID) {
		return nil
	}
	used, err := l.client.Get(ctx, l.key(workspaceID, time.Now())).Int64()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		slog.WarnContext(ctx, "ai spend limiter check failed, allowing request", "error", err)
		return nil
	}
	if used >= int64(l.dailyBudget) {
		metricSpendLimitRejections.Inc()
		return ErrSpendLimitExceeded
	}
	return nil
}

// Record adds tokens to the workspace's daily counter. Best-effort: errors are
// logged, not returned. The key expires after ~2 days so counters self-clean.
func (l *SpendLimiter) Record(ctx context.Context, workspaceID string, tokens int) {
	if !l.enabled(workspaceID) || tokens <= 0 {
		return
	}
	key := l.key(workspaceID, time.Now())
	pipe := l.client.TxPipeline()
	pipe.IncrBy(ctx, key, int64(tokens))
	pipe.Expire(ctx, key, 48*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.WarnContext(ctx, "ai spend limiter record failed", "workspace_id", workspaceID, "error", err)
	}
}
