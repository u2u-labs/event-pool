package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *Server) rateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user address from context (set by authMiddleware)
			address, ok := r.Context().Value("address").(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Create Redis key for this user
			key := fmt.Sprintf("rate_limit:%s", address)

			// Get current count
			ctx := r.Context()
			count, err := s.rdb.Get(ctx, key).Int()
			if err != nil && !errors.Is(err, redis.Nil) {
				s.logger.Errorf("Redis error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Check if limit exceeded
			if count >= limit {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Increment counter
			pipe := s.rdb.Pipeline()
			pipe.Incr(ctx, key)
			pipe.Expire(ctx, key, window)
			_, err = pipe.Exec(ctx)
			if err != nil {
				s.logger.Errorf("Redis pipeline error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count-1))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

			next.ServeHTTP(w, r)
		})
	}
}

// Alternative: Simple query counter (without rate limiting)
func (s *Server) queryCounterMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user address from context
			address, ok := r.Context().Value("address").(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Increment query counter
			ctx := r.Context()
			key := fmt.Sprintf("query_count:%s", address)

			// Increment counter with daily expiration
			pipe := s.rdb.Pipeline()
			pipe.Incr(ctx, key)
			pipe.Expire(ctx, key, 24*time.Hour)
			results, err := pipe.Exec(ctx)

			if err != nil {
				s.logger.Errorf("Redis error: %v", err)
				// Continue without failing the request
			} else if len(results) > 0 {
				count := results[0].(*redis.IntCmd).Val()
				w.Header().Set("X-Query-Count", fmt.Sprintf("%d", count))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Custom ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

// Modified rate limit middleware that only counts successful requests
func (s *Server) successBasedRateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user address from context (set by authMiddleware)
			address, ok := r.Context().Value("address").(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Create Redis key for this user
			key := fmt.Sprintf("rate_limit:%s", address)

			// Get current count
			ctx := r.Context()
			count, err := s.rdb.Get(ctx, key).Int()
			if err != nil && !errors.Is(err, redis.Nil) {
				s.logger.Errorf("Redis error: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Check if limit exceeded
			if count >= limit {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Set rate limit headers before processing
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count))

			// Wrap the response writer to capture status code
			wrappedWriter := &statusResponseWriter{
				ResponseWriter: w,
				statusCode:     0,
			}

			// Process the request
			next.ServeHTTP(wrappedWriter, r)

			// Only increment counter if request was successful (200-299)
			if wrappedWriter.statusCode >= 200 && wrappedWriter.statusCode < 300 {
				pipe := s.rdb.Pipeline()
				pipe.Incr(ctx, key)
				pipe.Expire(ctx, key, window)
				_, err = pipe.Exec(ctx)
				if err != nil {
					s.logger.Errorf("Redis pipeline error after successful request: %v", err)
					// Don't return error here as the request was already processed successfully
				}

				// Update the remaining count header after successful increment
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count-1))
			}
		})
	}
}
