package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// PublicEmbedHeaders relaxes the frame-blocking headers on anonymous embed
// routes only. It runs INSIDE the strict SecurityHeaders middleware, so it
// overrides the already-set values for this route group alone.
func PublicEmbedHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Del("X-Frame-Options")
		h.Set("Content-Security-Policy", "frame-ancestors *")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

// PublicRateLimiter throttles anonymous share traffic per token+IP using a
// fixed one-minute Dragonfly bucket (INCR+EXPIRE, same shape as the auth
// service limiter). Fails open when redis is unavailable.
type PublicRateLimiter struct {
	client    *redis.Client
	perMinute int
}

// NewPublicRateLimiter builds a limiter; a nil client disables limiting.
func NewPublicRateLimiter(client *redis.Client, perMinute int) *PublicRateLimiter {
	return &PublicRateLimiter{client: client, perMinute: perMinute}
}

// PublicRateKey derives the throttle key: hashed share token + client IP.
func PublicRateKey(r *http.Request) string {
	tok := chi.URLParam(r, "token")
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])[:16] + ":" + clientIP(r)
}

func clientIP(r *http.Request) string {
	// RealIP middleware (ApplyBaseMiddleware) already rewrites RemoteAddr.
	return r.RemoteAddr
}

// Middleware enforces the per-minute budget.
func (rl *PublicRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.client == nil || rl.perMinute <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			bucket := time.Now().Unix() / 60
			key := fmt.Sprintf("pubshare:rl:%s:%d", PublicRateKey(r), bucket)
			ctx := r.Context()
			count, err := rl.client.Incr(ctx, key).Result()
			if err != nil {
				next.ServeHTTP(w, r) // fail open, matching auth limiter
				return
			}
			if count == 1 {
				_ = rl.client.Expire(ctx, key, time.Minute).Err()
			}
			if count > int64(rl.perMinute) {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too_many_requests"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
