package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ajesus37/hCTF2/internal/auth"
	"github.com/ajesus37/hCTF2/internal/database"
)

type ProfileHandler struct {
	db *database.DB
}

func NewProfileHandler(db *database.DB) *ProfileHandler {
	return &ProfileHandler{db: db}
}

// ViewOwnProfile shows current user's profile
func (h *ProfileHandler) ViewOwnProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	stats, err := h.db.GetUserStats(claims.UserID)
	if err != nil {
		http.Error(w, "Error loading profile", http.StatusInternalServerError)
		return
	}

	submissions, _ := h.db.GetUserRecentSubmissions(claims.UserID, 20)
	solved, _ := h.db.GetUserSolvedChallenges(claims.UserID)

	data := map[string]interface{}{
		"Title":               "My Profile",
		"Page":                "profile",
		"User":                claims,
		"Stats":               stats,
		"RecentSubmissions":   submissions,
		"SolvedChallenges":    solved,
		"IsOwnProfile":        true,
	}

	// Render using server's render function
	_ = data // Will be used by main.go template rendering
}

// ViewUserProfile shows another user's profile (public)
func (h *ProfileHandler) ViewUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	stats, err := h.db.GetUserStats(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	submissions, _ := h.db.GetUserRecentSubmissions(userID, 20)
	solved, _ := h.db.GetUserSolvedChallenges(userID)

	data := map[string]interface{}{
		"Title":               "User Profile",
		"Page":                "profile",
		"User":                claims,
		"Stats":               stats,
		"RecentSubmissions":   submissions,
		"SolvedChallenges":    solved,
		"IsOwnProfile":        false,
	}

	_ = data // Will be used by main.go template rendering
}
