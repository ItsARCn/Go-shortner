package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/service"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// HandleAdminOverview handles GET /api/admin/overview
func (h *AdminHandler) HandleAdminOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := h.adminService.GetOverviewStats()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load overview statistics")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// HandleAdminUsers handles GET /api/admin/users
func (h *AdminHandler) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	role := r.URL.Query().Get("role")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	users, total, err := h.adminService.ListUsers(search, role, status, page, 20)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
	})
}

// HandleAdminTimeoutUser handles POST /api/admin/users/{id}/timeout
func (h *AdminHandler) HandleAdminTimeoutUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			id = parts[3]
		}
	}

	var req models.TimeoutUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	until, err := h.adminService.TimeoutUser(id, req.Duration, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrCannotModerateAdmin) {
			writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "User placed on timeout",
		"timeout_until": until,
	})
}

// HandleAdminBanUser handles POST /api/admin/users/{id}/ban
func (h *AdminHandler) HandleAdminBanUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			id = parts[3]
		}
	}

	var req models.BanUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.adminService.BanUser(id, req.Reason, req.DisableLinks); err != nil {
		if errors.Is(err, service.ErrCannotModerateAdmin) {
			writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "User has been banned",
	})
}

// HandleAdminUnbanUser handles POST /api/admin/users/{id}/unban
func (h *AdminHandler) HandleAdminUnbanUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			id = parts[3]
		}
	}

	if err := h.adminService.UnbanUser(id); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "User unbanned and restored to active status",
	})
}

// HandleAdminLinks handles GET /api/admin/links
func (h *AdminHandler) HandleAdminLinks(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	links, total, err := h.adminService.ListLinks(search, status, page, 25)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load links")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"links": links,
		"total": total,
		"page":  page,
	})
}

// HandleAdminDisableLink handles POST /api/admin/links/{code}/disable
func (h *AdminHandler) HandleAdminDisableLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	if err := h.adminService.DisableLink(code); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Link has been disabled",
	})
}

// HandleAdminEnableLink handles POST /api/admin/links/{code}/enable
func (h *AdminHandler) HandleAdminEnableLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	if err := h.adminService.EnableLink(code); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Link has been activated",
	})
}

// HandleAdminDeleteLink handles DELETE /api/admin/links/{code}
func (h *AdminHandler) HandleAdminDeleteLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			code = parts[3]
		}
	}

	if err := h.adminService.DeleteLink(code); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Link deleted",
	})
}

// HandleAdminReports handles GET /api/admin/reports
func (h *AdminHandler) HandleAdminReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	reports, total, err := h.adminService.ListReports(status, page, 25)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load reports")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reports": reports,
		"total":   total,
		"page":    page,
	})
}

// HandleAdminResolveReport handles POST /api/admin/reports/{id}/resolve
func (h *AdminHandler) HandleAdminResolveReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			id = parts[3]
		}
	}

	var req struct {
		Status string `json:"status"` // "reviewed" or "dismissed"
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.adminService.ResolveReport(id, req.Status); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Report status updated",
	})
}

// HandleAdminLoginRecords handles GET /api/admin/login-records
func (h *AdminHandler) HandleAdminLoginRecords(w http.ResponseWriter, r *http.Request) {
	result := r.URL.Query().Get("result")
	method := r.URL.Query().Get("method")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	records, total, err := h.adminService.GetLoginAuditRecords(result, method, page, 30)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load login audit records")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"records": records,
		"total":   total,
		"page":    page,
	})
}

// HandlePublicReport handles POST /api/reports
func (h *AdminHandler) HandlePublicReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	clientIP := middleware.ClientIP(r)

	if err := h.adminService.SubmitReport(req.ShortCode, req.Reason, req.Details, clientIP); err != nil {
		if errors.Is(err, service.ErrLinkNotFound) {
			writeJSONError(w, http.StatusNotFound, "Short link not found")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Abuse report submitted. Thank you for helping keep the platform safe.",
	})
}
