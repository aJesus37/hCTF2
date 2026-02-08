package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
)

type ChallengeHandler struct {
	db *database.DB
}

func NewChallengeHandler(db *database.DB) *ChallengeHandler {
	return &ChallengeHandler{db: db}
}

func (h *ChallengeHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	visibleOnly := claims == nil || !claims.IsAdmin

	challenges, err := h.db.GetChallenges(visibleOnly)
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenges)
}

func (h *ChallengeHandler) GetChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	challenge, err := h.db.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	questions, err := h.db.GetQuestionsByChallengeID(id)
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Remove flag from questions for non-admin users
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || !claims.IsAdmin {
		for i := range questions {
			questions[i].Flag = ""
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"challenge": challenge,
		"questions": questions,
	})
}

type SubmitFlagRequest struct {
	Flag string `json:"flag"`
}

func (h *ChallengeHandler) SubmitFlag(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	if claims == nil {
		w.Write([]byte(`<div class="text-red-400">Unauthorized</div>`))
		return
	}

	// Check if already solved
	solved, err := h.db.HasUserSolved(questionID, claims.UserID)
	if err != nil {
		w.Write([]byte(`<div class="text-red-400">Database error</div>`))
		return
	}
	if solved {
		w.Write([]byte(`<div class="text-yellow-400">You have already solved this question</div>`))
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.Write([]byte(`<div class="text-red-400">Invalid request</div>`))
		return
	}

	req := SubmitFlagRequest{
		Flag: r.FormValue("flag"),
	}

	// Get question
	question, err := h.db.GetQuestionByID(questionID)
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	// Validate flag
	submittedFlag := strings.TrimSpace(req.Flag)
	correctFlag := question.Flag
	if !question.CaseSensitive {
		submittedFlag = strings.ToLower(submittedFlag)
		correctFlag = strings.ToLower(correctFlag)
	}

	isCorrect := submittedFlag == correctFlag

	// Get user's team ID
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// Record submission
	if err := h.db.CreateSubmission(questionID, claims.UserID, user.TeamID, req.Flag, isCorrect); err != nil {
		http.Error(w, "Failed to record submission", http.StatusInternalServerError)
		return
	}

	// Return HTMX-friendly HTML response
	if isCorrect {
		w.Write([]byte(fmt.Sprintf(`<div class="text-green-400">✅ Correct! You earned %d points</div>`, question.Points)))
	} else {
		w.Write([]byte(`<div class="text-red-400">❌ Incorrect, try again</div>`))
	}
}

type CreateChallengeRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Difficulty  string  `json:"difficulty"`
	Tags        *string `json:"tags,omitempty"`
	Visible     bool    `json:"visible"`
}

func (h *ChallengeHandler) CreateChallenge(w http.ResponseWriter, r *http.Request) {
	var req CreateChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	challenge, err := h.db.CreateChallenge(req.Name, req.Description, req.Category, req.Difficulty, req.Tags, req.Visible)
	if err != nil {
		http.Error(w, "Failed to create challenge", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenge)
}

func (h *ChallengeHandler) UpdateChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CreateChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateChallenge(id, req.Name, req.Description, req.Category, req.Difficulty, req.Tags, req.Visible); err != nil {
		http.Error(w, "Failed to update challenge", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Challenge updated"))
}

func (h *ChallengeHandler) DeleteChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.db.DeleteChallenge(id); err != nil {
		http.Error(w, "Failed to delete challenge", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Challenge deleted"))
}

type CreateQuestionRequest struct {
	ChallengeID   string  `json:"challenge_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Flag          string  `json:"flag"`
	FlagMask      *string `json:"flag_mask,omitempty"`
	CaseSensitive bool    `json:"case_sensitive"`
	Points        int     `json:"points"`
	FileURL       *string `json:"file_url,omitempty"`
}

func (h *ChallengeHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	question, err := h.db.CreateQuestion(req.ChallengeID, req.Name, req.Description, req.Flag, req.FlagMask, req.CaseSensitive, req.Points, req.FileURL)
	if err != nil {
		http.Error(w, "Failed to create question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(question)
}

func (h *ChallengeHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateQuestion(id, req.Name, req.Description, req.Flag, req.FlagMask, req.CaseSensitive, req.Points, req.FileURL); err != nil {
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Question updated"))
}

func (h *ChallengeHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.db.DeleteQuestion(id); err != nil {
		http.Error(w, "Failed to delete question", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Question deleted"))
}
