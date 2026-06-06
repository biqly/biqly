package auth

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/security"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redisClient *redis.Client
}

func NewRateLimiter(r *redis.Client) *RateLimiter {
	return &RateLimiter{redisClient: r}
}

func (rl *RateLimiter) Limit(limit int, window time.Duration, keyPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.redisClient == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := getIP(r)
			// Create a 1-minute window bucket key
			bucket := time.Now().Unix() / int64(window.Seconds())
			key := fmt.Sprintf("ratelimit:%s:%s:%d", keyPrefix, ip, bucket)

			ctx := r.Context()
			count, err := rl.redisClient.Incr(ctx, key).Result()
			if err != nil {
				slog.Error("rate limit check redis error, bypass", "err", err)
				next.ServeHTTP(w, r)
				return
			}

			if count == 1 {
				if err := rl.redisClient.Expire(ctx, key, window).Err(); err != nil {
					slog.Warn("rate limit expire key error", "key", key, "err", err)
				}
			}

			if count > int64(limit) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				if _, err := w.Write([]byte(`{"error":"too_many_requests","message":"Rate limit exceeded. Please try again later."}`)); err != nil {
					_ = err
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if security.IsTrustedProxy(host) {
		if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
			parts := strings.Split(ip, ",")
			return strings.TrimSpace(parts[0])
		}
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return ip
		}
	}
	return host
}
