package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// HandleRegister handles POST /api/auth/register
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	clientIP := middleware.ClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	resp, err := h.authService.Register(req, clientIP, userAgent)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	setSessionCookie(w, resp.Token)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleLogin handles POST /api/auth/login
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	clientIP := middleware.ClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	resp, err := h.authService.Login(req, clientIP, userAgent)
	if err != nil {
		if errors.Is(err, service.ErrAccountBanned) {
			writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	setSessionCookie(w, resp.Token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleLogout handles POST /api/auth/logout
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "go_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

// HandleMe handles GET /api/auth/me
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "go_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(72 * time.Hour),
		MaxAge:   72 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
