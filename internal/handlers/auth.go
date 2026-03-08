package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ajesus37/hCTF2/internal/auth"
	"github.com/ajesus37/hCTF2/internal/database"
	"github.com/ajesus37/hCTF2/internal/email"
)

type AuthHandler struct {
	db       *database.DB
	emailSvc *email.Service
	baseURL  string
}

func NewAuthHandler(db *database.DB, emailSvc *email.Service, baseURL string) *AuthHandler {
	return &AuthHandler{db: db, emailSvc: emailSvc, baseURL: baseURL}
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

// Register godoc
// @Summary Register a new user account
// @Tags Auth
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param email formData string true "User email"
// @Param password formData string true "User password"
// @Param name formData string true "Display name"
// @Success 200 {object} object{user=object,token=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /auth/register [post]
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

// Login godoc
// @Summary Authenticate a user and set session cookie
// @Tags Auth
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param email formData string true "User email"
// @Param password formData string true "User password"
// @Success 200 {object} object{user=object,token=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/login [post]
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

// Logout godoc
// @Summary Clear authentication cookie and end session
// @Tags Auth
// @Produce plain
// @Security CookieAuth
// @Success 200 {string} string "Logged out"
// @Router /auth/logout [post]
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

// ForgotPassword godoc
// @Summary Request a password reset token
// @Description Generates a reset token and sends email via configured SMTP.
// @Tags Auth
// @Accept application/x-www-form-urlencoded
// @Produce plain
// @Param email formData string true "User email"
// @Success 200 {string} string "If that email exists, a reset link has been sent."
// @Failure 400 {object} object{error=string}
// @Router /auth/forgot-password [post]
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

	// Send reset email (or log link in dev mode)
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.baseURL, tokenStr)
	if err := h.emailSvc.SendPasswordReset(email, resetURL); err != nil {
		log.Printf("Failed to send password reset email: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("If that email exists, a reset link has been sent."))
}

// ResetPassword godoc
// @Summary Reset password using a valid reset token
// @Tags Auth
// @Accept application/x-www-form-urlencoded
// @Produce plain
// @Param token formData string true "Reset token"
// @Param password formData string true "New password"
// @Param confirm_password formData string true "Confirm new password"
// @Success 200 {string} string "Password reset successful. You can now login."
// @Failure 400 {object} object{error=string}
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Invalid request</p>`))
		return
	}

	token := r.FormValue("token")
	newPassword := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if token == "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Token is required</p>`))
		return
	}

	if newPassword == "" || confirmPassword == "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Password is required</p>`))
		return
	}

	if newPassword != confirmPassword {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Passwords do not match</p>`))
		return
	}

	// Validate token
	user, err := h.db.GetUserByResetToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Invalid or expired token</p>`))
		return
	}

	// Hash new password
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Error processing password</p>`))
		return
	}

	// Update password
	if err := h.db.UpdatePassword(user.ID, passwordHash); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Error updating password</p>`))
		return
	}

	// Clear token
	if err := h.db.ClearPasswordResetToken(user.ID); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<p class="text-red-500 dark:text-red-400">Error clearing token</p>`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<p class="text-green-500 dark:text-green-400">Password reset successful! <a href="/login" class="underline">Click here to login</a>.</p>`))
}
