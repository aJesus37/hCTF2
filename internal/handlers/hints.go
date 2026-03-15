package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ajesus37/hCTF2/internal/auth"
	"github.com/ajesus37/hCTF2/internal/database"
)

type HintHandler struct {
	db *database.DB
}

func NewHintHandler(db *database.DB) *HintHandler {
	return &HintHandler{db: db}
}

// UnlockHint godoc
// @Summary Unlock a hint for a question (costs points)
// @Tags Hints
// @Produce html
// @Security CookieAuth
// @Param id path string true "Hint ID"
// @Success 200 {string} string "Empty response; triggers HX-Trigger: refreshHints"
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /hints/{id}/unlock [post]
func (h *HintHandler) UnlockHint(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
		return
	}

	hintID := chi.URLParam(r, "id")

	// Get hint to find question_id
	hint, err := h.db.GetHintByID(hintID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Hint not found"}`))
		return
	}

	// Check if user already solved this question
	solved, err := h.db.HasUserSolved(hint.QuestionID, claims.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Error checking solve status"}`))
		return
	}
	if solved {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Cannot unlock hints for already solved questions"}`))
		return
	}

	// Check if already unlocked
	unlocked, err := h.db.IsHintUnlocked(hintID, claims.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Failed to check unlock status"}`))
		return
	}

	if unlocked {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Hint already unlocked"}`))
		return
	}

	// Get user's current team to record it with the hint unlock
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Failed to get user"}`))
		return
	}

	// Unlock hint
	if err := h.db.UnlockHint(hintID, claims.UserID, user.TeamID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Failed to unlock hint"}`))
		return
	}

	// Return a signal to refresh the hints container
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("HX-Trigger", "refreshHints")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// GetHints godoc
// @Summary Get hints for a question as HTML fragments (for HTMX) or JSON (for API clients)
// @Description Returns hints with unlock status. Send Accept: application/json for JSON response.
// @Tags Hints
// @Produce html,json
// @Param questionId path string true "Question ID"
// @Success 200 {string} string "HTML fragments with hint cards, or JSON array"
// @Failure 500 {object} object{error=string}
// @Router /questions/{questionId}/hints [get]
func (h *HintHandler) GetHints(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.GetHintsJSON(w, r)
		return
	}
	questionID := chi.URLParam(r, "questionId")
	claims := auth.GetUserFromContext(r.Context())

	hints, err := h.db.GetHintsByQuestionID(questionID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<p class="text-gray-400 text-sm">No hints available</p>`))
		return
	}

	if len(hints) == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<p class="text-gray-400 text-sm">No hints available</p>`))
		return
	}

	// Check if question is already solved
	var questionSolved bool
	if claims != nil {
		questionSolved, _ = h.db.HasUserSolved(questionID, claims.UserID)
	}

	// If authenticated, get unlock status
	var unlockedIDs []string
	if claims != nil {
		unlockedIDs, _ = h.db.GetUserUnlockedHints(claims.UserID, questionID)
	}

	// Build map for quick lookup
	unlockedMap := make(map[string]bool)
	for _, id := range unlockedIDs {
		unlockedMap[id] = true
	}

	// Build HTML response
	w.Header().Set("Content-Type", "text/html")
	for _, hint := range hints {
		if unlockedMap[hint.ID] {
			// Show unlocked hint with content (preserve line breaks)
			fmt.Fprintf(w, `<div class="p-3 bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-700 rounded text-blue-900 dark:text-blue-100 text-sm">
                <strong>Hint %d:</strong> <pre class="whitespace-pre-wrap font-sans mt-1">%s</pre>
                <span class="text-xs text-blue-600 dark:text-blue-300">(Cost: %d points)</span>
            </div>`, hint.Order, hint.Content, hint.Cost)
		} else {
			// Show locked hint with unlock button
			if claims != nil {
				if questionSolved {
					// Question already solved - disable unlock button
					fmt.Fprintf(w, `<div class="p-3 bg-gray-100 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded text-gray-600 dark:text-gray-400 text-sm flex justify-between items-center">
                        <span><strong>Hint %d</strong> (Cost: %d points)</span>
                        <button disabled
                            class="px-3 py-1 bg-gray-300 dark:bg-gray-700 text-gray-500 dark:text-gray-500 text-xs rounded cursor-not-allowed">
                            Unavailable
                        </button>
                    </div>`, hint.Order, hint.Cost)
				} else {
					// Question not solved - show unlock button
					fmt.Fprintf(w, `<div class="p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded text-amber-900 dark:text-amber-100 text-sm flex justify-between items-center">
                        <span><strong>Hint %d</strong> (Cost: %d points)</span>
                        <button hx-post="/api/hints/%s/unlock"
                            hx-swap="none"
                            class="px-3 py-1 bg-amber-500 hover:bg-amber-600 text-white text-xs rounded transition">
                            Unlock
                        </button>
                    </div>`, hint.Order, hint.Cost, hint.ID)
				}
			} else {
				// Show locked hint without unlock button (not authenticated)
				fmt.Fprintf(w, `<div class="p-3 bg-gray-100 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded text-gray-700 dark:text-gray-300 text-sm">
                    <strong>Hint %d</strong> (Cost: %d points)
                    <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">Login to unlock</span>
                </div>`, hint.Order, hint.Cost)
			}
		}
	}
}

// GetHintsJSON returns hints as JSON (for API clients)
func (h *HintHandler) GetHintsJSON(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "questionId")
	claims := auth.GetUserFromContext(r.Context())

	hints, err := h.db.GetHintsByQuestionID(questionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	// If authenticated, get unlock status
	var unlockedIDs []string
	if claims != nil {
		unlockedIDs, _ = h.db.GetUserUnlockedHints(claims.UserID, questionID)
	}

	// Build response with unlock status
	unlockedMap := make(map[string]bool)
	for _, id := range unlockedIDs {
		unlockedMap[id] = true
	}

	response := make([]map[string]interface{}, len(hints))
	for i, hint := range hints {
		response[i] = map[string]interface{}{
			"id":       hint.ID,
			"order":    hint.Order,
			"cost":     hint.Cost,
			"unlocked": unlockedMap[hint.ID],
		}

		// Only include content if unlocked
		if unlockedMap[hint.ID] {
			response[i]["content"] = hint.Content
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
