package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

type UserHandler struct {
	linkService *service.LinkService
	authService *service.AuthService
}

func NewUserHandler(linkService *service.LinkService, authService *service.AuthService) *UserHandler {
	return &UserHandler{
		linkService: linkService,
		authService: authService,
	}
}

// HandleDashboard handles GET /api/user/dashboard
func (h *UserHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	dash, err := h.linkService.GetUserDashboard(user.ID, user.QuotaLimit, search, status, page, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dash)
}

// HandleLinks handles GET /api/user/links
func (h *UserHandler) HandleLinks(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	dash, err := h.linkService.GetUserDashboard(user.ID, user.QuotaLimit, search, status, page, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to fetch links")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dash)
}

// HandleRenewLink handles POST /api/user/links/{code}/renew
func (h *UserHandler) HandleRenewLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	var req models.RenewLinkRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Expiration == "" {
		req.Expiration = "30d" // default renewal
	}

	link, err := h.linkService.RenewLink(code, claims.UserID, req.Expiration)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			writeJSONError(w, http.StatusForbidden, "You do not have permission to renew this link")
			return
		}
		if errors.Is(err, service.ErrLinkNotFound) {
			writeJSONError(w, http.StatusNotFound, "Link not found")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Link renewed successfully",
		"short_code": link.ShortCode,
		"expires_at": link.ExpiresAt,
		"status":     link.Status,
	})
}

// HandleDeleteLink handles DELETE /api/user/links/{code}
func (h *UserHandler) HandleDeleteLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	err := h.linkService.DeleteLink(code, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			writeJSONError(w, http.StatusForbidden, "You do not have permission to delete this link")
			return
		}
		if errors.Is(err, service.ErrLinkNotFound) {
			writeJSONError(w, http.StatusNotFound, "Link not found")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Link deleted successfully"})
}

// HandleLinkAnalytics handles GET /api/user/links/{code}/analytics
func (h *UserHandler) HandleLinkAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	analytics, err := h.linkService.GetLinkAnalytics(code, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			writeJSONError(w, http.StatusForbidden, "You do not have permission to view analytics for this link")
			return
		}
		if errors.Is(err, service.ErrLinkNotFound) {
			writeJSONError(w, http.StatusNotFound, "Link not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to load analytics")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(analytics)
}
