package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
)

type HintHandler struct {
	db *database.DB
}

func NewHintHandler(db *database.DB) *HintHandler {
	return &HintHandler{db: db}
}

// UnlockHint handles hint unlock requests
func (h *HintHandler) UnlockHint(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Unauthorized"}`))
		return
	}

	hintID := chi.URLParam(r, "id")

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

	// Unlock hint
	if err := h.db.UnlockHint(hintID, claims.UserID); err != nil {
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

// GetHints returns hints for a question (with unlock status) as HTML for HTMX
func (h *HintHandler) GetHints(w http.ResponseWriter, r *http.Request) {
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
			fmt.Fprintf(w, `<div class="p-3 bg-green-900 border border-green-700 rounded text-green-100 text-sm">
                <strong>Hint %d:</strong> <pre class="whitespace-pre-wrap font-sans">%s</pre>
                <span class="ml-2 text-xs opacity-75">(Cost: %d points)</span>
            </div>`, hint.Order, hint.Content, hint.Cost)
		} else {
			// Show locked hint with unlock button
			if claims != nil {
				fmt.Fprintf(w, `<div class="p-3 bg-gray-700 border border-gray-600 rounded text-gray-200 text-sm flex justify-between items-center">
                    <span><strong>Hint %d</strong> (Cost: %d points)</span>
                    <button hx-post="/api/hints/%s/unlock"
                        hx-swap="none"
                        class="px-3 py-1 bg-yellow-600 hover:bg-yellow-700 text-white text-xs rounded transition">
                        Unlock
                    </button>
                </div>`, hint.Order, hint.Cost, hint.ID)
			} else {
				// Show locked hint without unlock button (not authenticated)
				fmt.Fprintf(w, `<div class="p-3 bg-gray-700 border border-gray-600 rounded text-gray-200 text-sm">
                    <strong>Hint %d</strong> (Cost: %d points)
                    <span class="ml-2 text-xs text-gray-400">Login to unlock</span>
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
