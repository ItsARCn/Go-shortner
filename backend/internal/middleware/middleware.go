package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/service"
)

type contextKey string

const UserContextKey contextKey = "user_claims"

// ClientIP extracts client IP, honoring Cloudflare tunnel and proxy headers safely.
func ClientIP(r *http.Request) string {
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SecurityHeaders adds modern security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs incoming requests concisely.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			log.Printf("%s %s %s (%s)", r.Method, r.URL.Path, ClientIP(r), time.Since(start))
		}
	})
}

// RateLimiter implements an in-memory token bucket per client IP.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientBucket
	rate    int
	window  time.Duration
}

type clientBucket struct {
	tokens    int
	lastReset time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientBucket),
		rate:    rate,
		window:  window,
	}

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-2 * window)
			for ip, b := range rl.clients {
				if b.lastReset.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r)

		rl.mu.Lock()
		b, exists := rl.clients[ip]
		now := time.Now()

		if !exists || now.Sub(b.lastReset) > rl.window {
			rl.clients[ip] = &clientBucket{
				tokens:    rl.rate - 1,
				lastReset: now,
			}
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if b.tokens <= 0 {
			rl.mu.Unlock()
			http.Error(w, `{"error":"Too many requests. Please slow down."}`, http.StatusTooManyRequests)
			return
		}

		b.tokens--
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	}
}

// ExtractToken retrieves JWT from cookie or Authorization header.
func ExtractToken(r *http.Request) string {
	// 1. Check HTTP-only cookie first
	if cookie, err := r.Cookie("go_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// 2. Check Authorization Bearer header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// RequireAuth enforces authenticated access and populates claims in context.
func RequireAuth(authService *service.AuthService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ExtractToken(r)
		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
			return
		}

		claims, err := authService.VerifyToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid or expired session"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// OptionalAuth attaches claims to context if valid, otherwise proceeds anonymously.
func OptionalAuth(authService *service.AuthService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ExtractToken(r)
		if tokenStr != "" {
			if claims, err := authService.VerifyToken(tokenStr); err == nil {
				ctx := context.WithValue(r.Context(), UserContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

// GetUserFromContext extracts token claims from request context if present.
func GetUserFromContext(ctx context.Context) *models.TokenClaims {
	if val := ctx.Value(UserContextKey); val != nil {
		if claims, ok := val.(*models.TokenClaims); ok {
			return claims
		}
	}
	return nil
}

// RequireRole enforces role-based access control (super_admin or moderator).
// If unauthorized, an audit record is logged and HTTP 403 is returned.
func RequireRole(authService *service.AuthService, minRole models.UserRole, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ExtractToken(r)
		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
			return
		}

		claims, err := authService.VerifyToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid or expired session"}`))
			return
		}

		user, err := authService.GetUserByID(claims.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"User account not found"}`))
			return
		}

		isAllowed := false
		if user.Role == models.RoleSuperAdmin {
			isAllowed = true
		} else if user.Role == models.RoleModerator && minRole != models.RoleSuperAdmin {
			isAllowed = true
		}

		if !isAllowed {
			clientIP := ClientIP(r)
			userAgent := r.Header.Get("User-Agent")
			authService.RecordUnauthorizedAudit(user.Email, "admin_access", clientIP, userAgent)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"You are not authorized to access the admin panel"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// Recoverer catches panics gracefully.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Printf("[PANIC] recovered: %v", rvr)
				http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger is an alias for LoggingMiddleware.
func Logger(next http.Handler) http.Handler {
	return LoggingMiddleware(next)
}
