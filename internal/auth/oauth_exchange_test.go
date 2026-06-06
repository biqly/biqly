package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestOAuthCallbackCodeIssueAndRedeem(t *testing.T) {
	redisDSN := os.Getenv("BI_AUTH_REDIS_DSN")
	if redisDSN == "" {
		redisDSN = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(redisDSN)
	if err != nil {
		t.Skip("invalid redis URL:", err)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("redis not available:", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			t.Errorf("rdb.Close() error = %v", err)
		}
	}()

	svc := &Service{redisClient: rdb}
	resp := &TokenResponse{
		AccessToken:  "access",
		RefreshToken: "refresh",
		UserID:       "user-1",
		Email:        "u@example.com",
		Roles:        []string{"viewer"},
	}

	code, err := svc.IssueOAuthCallbackCode(ctx, resp)
	if err != nil {
		t.Fatalf("IssueOAuthCallbackCode() error = %v", err)
	}
	if code == "" {
		t.Fatal("IssueOAuthCallbackCode() returned empty code")
	}

	got, err := svc.RedeemOAuthCallbackCode(ctx, code)
	if err != nil {
		t.Fatalf("RedeemOAuthCallbackCode() error = %v", err)
	}
	if got.AccessToken != resp.AccessToken || got.RefreshToken != resp.RefreshToken {
		t.Fatalf("redeemed tokens mismatch: %+v", got)
	}

	// Within the grace window the same code yields the same tokens so that
	// concurrent retries (e.g. StrictMode double-mount, network races) do not
	// bounce the user back to sign-in.
	regot, err := svc.RedeemOAuthCallbackCode(ctx, code)
	if err != nil {
		t.Fatalf("grace redeem error = %v, want success within grace window", err)
	}
	if regot.AccessToken != resp.AccessToken || regot.RefreshToken != resp.RefreshToken {
		t.Fatalf("grace redeem token mismatch: %+v", regot)
	}

	// After the grace TTL elapses the code is gone for good.
	if err := rdb.Del(ctx, oauthCallbackUsedKeyPrefix+code).Err(); err != nil {
		t.Fatalf("failed to delete grace key: %v", err)
	}
	_, err = svc.RedeemOAuthCallbackCode(ctx, code)
	if !errors.Is(err, ErrInvalidOAuthCallbackCode) {
		t.Fatalf("post-grace redeem error = %v, want ErrInvalidOAuthCallbackCode", err)
	}
}
