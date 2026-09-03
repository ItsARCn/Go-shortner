package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

func setupTestServer(t *testing.T) (*httptest.Server, *repository.LinkRepository, func()) {
	tmpDB, err := os.CreateTemp("", "go-short-api-*.sqlite")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmpDB.Close()

	db, err := database.InitDB(tmpDB.Name())
	if err != nil {
		os.Remove(tmpDB.Name())
		t.Fatalf("failed to init db: %v", err)
	}

	cfg := &config.Config{
		BaseURL:                     "http://test.local",
		AnonymousDailyQuota:         3, // Small quota for testing
		RegisteredMonthlyQuota:      100,
		AnonymousMaxExpirationDays:  7,
		RegisteredMaxExpirationDays: 365,
	}

	repo := repository.NewLinkRepository(db)
	svc := service.NewLinkService(repo, cfg)
	h := NewHandler(svc, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/links/shorten", h.HandleShorten)
	mux.HandleFunc("GET /api/links/{code}/info", h.HandleGetInfo)
	mux.HandleFunc("GET /api/health", h.HandleHealth)
	mux.HandleFunc("GET /{code}", h.HandleRedirect)

	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return server, repo, cleanup
}

func TestAPIHealth(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("failed to GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", body["status"])
	}
}

func TestAPIShortenAndRedirect(t *testing.T) {
	ts, repo, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Shorten valid URL
	payload := `{"url":"https://google.com/search","expiration":"7d"}`
	resp, err := http.Post(ts.URL+"/api/links/shorten", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/links/shorten failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(body))
	}

	var shortenResp models.ShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&shortenResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(shortenResp.ShortCode) != 6 {
		t.Errorf("expected 6 char code, got %q", shortenResp.ShortCode)
	}

	// 2. Test 302 Redirect
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirect
		},
	}

	redirResp, err := client.Get(ts.URL + "/" + shortenResp.ShortCode)
	if err != nil {
		t.Fatalf("GET /code failed: %v", err)
	}
	defer redirResp.Body.Close()

	if redirResp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 StatusFound, got %d", redirResp.StatusCode)
	}
	location := redirResp.Header.Get("Location")
	if location != "https://google.com/search" {
		t.Errorf("expected location https://google.com/search, got %q", location)
	}

	// 3. Test Expired Link Page
	// Force link to be expired in DB
	link, _ := repo.GetLinkByCode(shortenResp.ShortCode)
	_ = repo.UpdateLinkStatus(link.ID, models.StatusExpired)
	// Also set expires_at in the past
	_, _ = ts.Client().Post(ts.URL+"/api/links/shorten", "application/json", bytes.NewBuffer(nil))

	expiredResp, err := http.Get(ts.URL + "/" + shortenResp.ShortCode)
	if err != nil {
		t.Fatalf("GET expired link failed: %v", err)
	}
	defer expiredResp.Body.Close()

	if expiredResp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 Gone for expired link, got %d", expiredResp.StatusCode)
	}
	htmlBytes, _ := io.ReadAll(expiredResp.Body)
	if !strings.Contains(string(htmlBytes), "This link has expired") {
		t.Errorf("expected 'This link has expired' in html output")
	}
}

func TestAPIShortenAbuseAndQuota(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Prohibited private address
	badPayload := `{"url":"http://127.0.0.1:8080/secret","expiration":"7d"}`
	resp, err := http.Post(ts.URL+"/api/links/shorten", "application/json", strings.NewReader(badPayload))
	if err != nil {
		t.Fatalf("failed request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for private IP, got %d", resp.StatusCode)
	}

	// 2. Test quota limit (configured to 3 in setupTestServer)
	for i := 1; i <= 3; i++ {
		p := `{"url":"https://example.com/ok","expiration":"1d"}`
		r, err := http.Post(ts.URL+"/api/links/shorten", "application/json", strings.NewReader(p))
		if err != nil {
			t.Fatalf("link %d failed: %v", i, err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusCreated {
			t.Fatalf("link %d expected 201, got %d", i, r.StatusCode)
		}
	}

	// 4th link must exceed anonymous quota (429 Too Many Requests)
	p4 := `{"url":"https://example.com/exceeded","expiration":"1d"}`
	r4, err := http.Post(ts.URL+"/api/links/shorten", "application/json", strings.NewReader(p4))
	if err != nil {
		t.Fatalf("link 4 request failed: %v", err)
	}
	defer r4.Body.Close()

	if r4.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", r4.StatusCode)
	}
}
