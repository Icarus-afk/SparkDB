package server

import (
	"log/slog"
	"net/http"
	"time"

	"sparkdb/internal/auth"
	"sparkdb/internal/query"
	"sparkdb/pkg/api"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rw.status, "duration", time.Since(start))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err)
				writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: "internal server error", Code: 500})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowOrigin := "*"
			if len(allowedOrigins) > 0 && allowedOrigins[0] != "*" {
				allowOrigin = ""
				for _, o := range allowedOrigins {
					if o == origin {
						allowOrigin = origin
						break
					}
				}
				if allowOrigin == "" {
					allowOrigin = allowedOrigins[0]
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE, PUT")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitMiddleware(userLimiter, ipLimiter *query.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ipLimiter.Allow(r.RemoteAddr) {
				writeJSON(w, http.StatusTooManyRequests, api.ErrorResponse{Error: "rate limit exceeded", Code: 429})
				return
			}
			key := r.RemoteAddr
			if user := auth.UserFromContext(r.Context()); user != nil {
				key = user.Username
			}
			if !userLimiter.Allow(key) {
				writeJSON(w, http.StatusTooManyRequests, api.ErrorResponse{Error: "rate limit exceeded", Code: 429})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(authenticator *auth.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authenticator.AuthenticateRequest(r)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: err.Error(), Code: 401})
				return
			}
			ctx := auth.ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func optionalAuthMiddleware(authenticator *auth.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authenticator.AuthenticateRequest(r)
			if err == nil && user != nil {
				ctx := auth.ContextWithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
