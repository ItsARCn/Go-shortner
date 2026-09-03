package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/service"
)

type Handler struct {
	linkService *service.LinkService
	startTime   time.Time
}

func NewHandler(linkService *service.LinkService) *Handler {
	return &Handler{
		linkService: linkService,
		startTime:   time.Now(),
	}
}

// HandleShorten handles POST /api/links/shorten
func (h *Handler) HandleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	clientIP := middleware.ClientIP(r)

	var ownerID *string
	if claims := middleware.GetUserFromContext(r.Context()); claims != nil {
		ownerID = &claims.UserID
	}

	resp, err := h.linkService.Shorten(req, ownerID, clientIP)
	if err != nil {
		if errors.Is(err, service.ErrAnonymousQuotaMet) || errors.Is(err, service.ErrRegisteredQuotaMet) {
			writeJSONError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleGetInfo handles GET /api/links/{code}/info
func (h *Handler) HandleGetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		// Fallback for older Go routing if needed
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 {
			code = parts[2]
		}
	}

	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing short code")
		return
	}

	info, err := h.linkService.GetInfo(code)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Short link not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// HandleRedirect handles GET /{code}
func (h *Handler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		code = strings.TrimPrefix(r.URL.Path, "/")
	}

	// Filter out non-shortcode root paths (e.g. static assets or api)
	if code == "" || code == "favicon.ico" || strings.HasPrefix(code, "api/") || strings.HasPrefix(code, "assets/") {
		http.NotFound(w, r)
		return
	}

	clientIP := middleware.ClientIP(r)
	referrer := r.Header.Get("Referer")
	userAgent := r.Header.Get("User-Agent")

	link, err := h.linkService.Resolve(code, clientIP, referrer, userAgent)
	if err != nil {
		if errors.Is(err, service.ErrLinkExpired) {
			// Render clean expired link notice page
			renderExpiredPage(w, link)
			return
		}
		if errors.Is(err, service.ErrLinkDisabled) {
			renderNoticePage(w, "Link Disabled", "This link has been deactivated for violating acceptable use policies.", http.StatusForbidden)
			return
		}
		renderNoticePage(w, "Link Not Found", "The requested short link does not exist or has been removed.", http.StatusNotFound)
		return
	}

	// Active link: perform 302 redirect directly
	http.Redirect(w, r, link.DestinationURL, http.StatusFound)
}

// HandleHealth handles GET /api/health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := map[string]interface{}{
		"status":          "healthy",
		"uptime_seconds":  int(time.Since(h.startTime).Seconds()),
		"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
		"goroutines":      runtime.NumGoroutine(),
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

var expiredTemplate = template.Must(template.New("expired").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Link Expired - GO Shortener</title>
	<style>
		:root {
			--bg: #090d16;
			--card: #111827;
			--border: #1f2937;
			--text: #f3f4f6;
			--muted: #9ca3af;
			--accent: #f43f5e;
		}
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			background-color: var(--bg);
			color: var(--text);
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 1.5rem;
		}
		.card {
			background-color: var(--card);
			border: 1px solid var(--border);
			border-radius: 12px;
			max-width: 440px;
			width: 100%;
			padding: 2.25rem;
			text-align: center;
			box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
		}
		.badge {
			display: inline-block;
			background: rgba(244, 63, 94, 0.15);
			color: var(--accent);
			font-size: 0.75rem;
			font-weight: 700;
			letter-spacing: 0.05em;
			padding: 0.35rem 0.75rem;
			border-radius: 9999px;
			margin-bottom: 1.25rem;
			text-transform: uppercase;
		}
		h1 {
			font-size: 1.5rem;
			font-weight: 600;
			margin-bottom: 0.5rem;
		}
		p.desc {
			color: var(--muted);
			font-size: 0.925rem;
			line-height: 1.5;
			margin-bottom: 1.75rem;
		}
		.meta-box {
			background: #0d121f;
			border: 1px solid var(--border);
			border-radius: 8px;
			padding: 1rem;
			text-align: left;
			font-size: 0.85rem;
			margin-bottom: 2rem;
		}
		.meta-row {
			display: flex;
			justify-content: space-between;
			padding: 0.35rem 0;
		}
		.meta-row span.label { color: var(--muted); }
		.meta-row span.val { font-weight: 500; font-family: ui-monospace, monospace; }
		.btn {
			display: inline-block;
			background: #2563eb;
			color: white;
			text-decoration: none;
			padding: 0.7rem 1.25rem;
			border-radius: 6px;
			font-size: 0.9rem;
			font-weight: 500;
			transition: background 0.15s ease;
		}
		.btn:hover { background: #1d4ed8; }
	</style>
</head>
<body>
	<div class="card">
		<div class="badge">Status: Expired</div>
		<h1>This link has expired</h1>
		<p class="desc">This link is no longer active and can no longer redirect to its original destination.</p>
		
		<div class="meta-box">
			<div class="meta-row">
				<span class="label">Short Code</span>
				<span class="val">{{.ShortCode}}</span>
			</div>
			<div class="meta-row">
				<span class="label">Created</span>
				<span class="val">{{.CreatedAtFormatted}}</span>
			</div>
			<div class="meta-row">
				<span class="label">Expired</span>
				<span class="val">{{.ExpiresAtFormatted}}</span>
			</div>
		</div>

		<a href="/" class="btn">Create a New Link</a>
	</div>
</body>
</html>`))

func renderExpiredPage(w http.ResponseWriter, l *models.Link) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusGone)

	data := struct {
		ShortCode          string
		CreatedAtFormatted string
		ExpiresAtFormatted string
	}{
		ShortCode:          l.ShortCode,
		CreatedAtFormatted: l.CreatedAt.Format("January 02, 2006"),
		ExpiresAtFormatted: l.ExpiresAt.Format("January 02, 2006"),
	}

	_ = expiredTemplate.Execute(w, data)
}

func renderNoticePage(w http.ResponseWriter, title, message string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>%s - GO Shortener</title>
	<style>
		body { background: #090d16; color: #f3f4f6; font-family: -apple-system, BlinkMacSystemFont, sans-serif; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.box { background: #111827; border: 1px solid #1f2937; padding: 2rem; border-radius: 10px; max-width: 400px; text-align: center; }
		h1 { font-size: 1.4rem; margin-bottom: 0.5rem; }
		p { color: #9ca3af; font-size: 0.9rem; margin-bottom: 1.5rem; }
		a { color: #3b82f6; text-decoration: none; font-size: 0.85rem; }
	</style>
</head>
<body>
	<div class="box">
		<h1>%s</h1>
		<p>%s</p>
		<a href="/">← Return to Homepage</a>
	</div>
</body>
</html>`, title, title, message)
	_, _ = w.Write([]byte(html))
}
