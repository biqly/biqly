package provider

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestRecordLLMRetry(t *testing.T) {
	t.Parallel()
	// Should not panic; calls observability.Default() singleton.
	recordLLMRetry("openai")
	recordLLMRetry("anthropic")
	recordLLMRetry("unknown-provider")
}

func TestRecordLLMError(t *testing.T) {
	t.Parallel()
	// Should not panic.
	recordLLMError("openai", errors.New("timeout"), 503)
	recordLLMError("anthropic", nil, 0)
	recordLLMError("openai", errors.New("unmarshal failed"), 200)
}

func TestNormalizeProviderName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"openai", "openai"},
		{"OpenAI", "openai"},
		{"OPENAI", "openai"},
		{"openai-compatible", "openai"},
		{"OpenAI-Compatible", "openai"},
		{"anthropic", "anthropic"},
		{"Anthropic", "anthropic"},
		{"ANTHROPIC", "anthropic"},
		{"", "other"},
		{"gemini", "other"},
		{"  openai  ", "openai"}, // whitespace trimmed
		{"  Anthropic  ", "anthropic"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := normalizeProviderName(tc.input); got != tc.want {
				t.Errorf("normalizeProviderName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsRetriableNetErrNil(t *testing.T) {
	t.Parallel()
	if isRetriableNetErr(nil) {
		t.Error("nil should not be retriable")
	}
}

func TestIsRetriableNetErrECONNRESET(t *testing.T) {
	t.Parallel()
	if !isRetriableNetErr(syscall.ECONNRESET) {
		t.Error("ECONNRESET should be retriable")
	}
}

func TestIsRetriableNetErrEPIPE(t *testing.T) {
	t.Parallel()
	if !isRetriableNetErr(syscall.EPIPE) {
		t.Error("EPIPE should be retriable")
	}
}

func TestIsRetriableNetErrETIMEDOUT(t *testing.T) {
	t.Parallel()
	if !isRetriableNetErr(syscall.ETIMEDOUT) {
		t.Error("ETIMEDOUT should be retriable")
	}
}

func TestIsRetriableNetErrDeadlineExceeded(t *testing.T) {
	t.Parallel()
	if !isRetriableNetErr(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be retriable")
	}
}

func TestIsRetriableNetErrNetTimeout(t *testing.T) {
	t.Parallel()
	err := &net.DNSError{Name: "example.com", IsTimeout: true}
	if !isRetriableNetErr(err) {
		t.Error("net.Error with Timeout()=true should be retriable")
	}
}

func TestIsRetriableNetErrNetOpError(t *testing.T) {
	t.Parallel()
	err := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	if !isRetriableNetErr(err) {
		t.Error("*net.OpError should be retriable")
	}
}

func TestIsRetriableNetErrPlainError(t *testing.T) {
	t.Parallel()
	if isRetriableNetErr(errors.New("some random error")) {
		t.Error("random error should not be retriable")
	}
}

func TestCryptoRandomUnitFloatSanity(t *testing.T) {
	t.Parallel()
	for range 100 {
		f := cryptoRandomUnitFloat()
		if f < 0 || f >= 1 {
			t.Fatalf("cryptoRandomUnitFloat() = %f, want in [0,1)", f)
		}
	}
}

func TestJitteredBackoffPositive(t *testing.T) {
	t.Parallel()
	for attempt := 1; attempt <= 4; attempt++ {
		d := jitteredBackoff(attempt)
		if d <= 0 {
			t.Fatalf("jitteredBackoff(%d) = %v, want > 0", attempt, d)
		}
	}
}

func TestSleepCtxCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); err == nil {
		t.Error("expected context error for cancelled ctx")
	}
}

func TestSleepCtxNormal(t *testing.T) {
	t.Parallel()
	start := time.Now()
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("sleepCtx: %v", err)
	}
	if time.Since(start) < time.Millisecond {
		t.Error("sleep returned too fast")
	}
}
