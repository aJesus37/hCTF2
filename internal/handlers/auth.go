package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
)

type AuthHandler struct {
	db *database.DB
}

func NewAuthHandler(db *database.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req := RegisterRequest{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
		Name:     r.FormValue("name"),
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create user
	user, err := h.db.CreateUser(req.Email, passwordHash, req.Name, false)
	if err != nil {
		http.Error(w, "Email already exists", http.StatusConflict)
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user.ID, user.Email, user.Name, user.IsAdmin)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(168 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req := LoginRequest{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	}

	// Get user
	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		// Check if this is an HTMX request
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// HTMX request - return 200 with error HTML so it gets swapped
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<p class="text-red-400 text-center">Invalid credentials - please try again</p>`))
		} else {
			// API request - return 401
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		}
		return
	}

	// Verify password
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		// Check if this is an HTMX request
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// HTMX request - return 200 with error HTML so it gets swapped
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<p class="text-red-400 text-center">Invalid credentials - please try again</p>`))
		} else {
			// API request - return 401
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		}
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user.ID, user.Email, user.Name, user.IsAdmin)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(168 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Check if this is an HTMX request (form-data) or API request (JSON)
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// HTMX form submission - redirect to home
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	} else {
		// API request - send JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user":  user,
			"token": token,
		})
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logged out"))
}

// ForgotPassword generates reset token and sends email (placeholder without SMTP)
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByEmail(email)
	if err != nil {
		// Don't reveal if email exists - always return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("If that email exists, a reset link has been sent."))
		return
	}

	// Generate secure token
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}
	tokenStr := hex.EncodeToString(token)

	// Store token with 30 minute expiration
	expires := time.Now().Add(30 * time.Minute)
	if err := h.db.CreatePasswordResetToken(user.ID, tokenStr, expires); err != nil {
		http.Error(w, "Error creating reset token", http.StatusInternalServerError)
		return
	}

	// TODO: Send email with reset link
	// For now, just return success message. In production, you would:
	// emailService.SendPasswordReset(email, tokenStr, "https://your-domain.com")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("If that email exists, a reset link has been sent."))
}

// ResetPassword validates token and updates password
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	newPassword := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	if newPassword == "" || confirmPassword == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	if newPassword != confirmPassword {
		http.Error(w, "Passwords do not match", http.StatusBadRequest)
		return
	}

	// Validate token
	user, err := h.db.GetUserByResetToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	// Hash new password
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	// Update password
	if err := h.db.UpdatePassword(user.ID, passwordHash); err != nil {
		http.Error(w, "Error updating password", http.StatusInternalServerError)
		return
	}

	// Clear token
	if err := h.db.ClearPasswordResetToken(user.ID); err != nil {
		http.Error(w, "Error clearing token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Password reset successful. You can now login."))
}
