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
		// Calculate actual points earned after hint deductions
		hintCost := 0
		if h.db != nil {
			cost, err := h.db.GetUserHintCostForQuestion(claims.UserID, questionID)
			if err == nil {
				hintCost = cost
			}
		}
		pointsEarned := question.Points - hintCost
		if pointsEarned < 0 {
			pointsEarned = 0
		}

		// Show hint cost info if hints were used
		if hintCost > 0 {
			w.Write([]byte(fmt.Sprintf(`<div class="text-green-400">✅ Correct! You earned %d points <span class="text-yellow-300 text-sm">(-%d hint cost)</span></div>`, pointsEarned, hintCost)))
		} else {
			w.Write([]byte(fmt.Sprintf(`<div class="text-green-400">✅ Correct! You earned %d points</div>`, pointsEarned)))
		}
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
	contentType := r.Header.Get("Content-Type")
	var req CreateChallengeRequest

	if strings.Contains(contentType, "application/json") {
		// JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`Invalid request`))
			return
		}
		req.Name = r.FormValue("name")
		req.Description = r.FormValue("description")
		// Multi-select categories: join with comma
		categories := r.Form["category"]
		req.Category = strings.Join(categories, ",")
		req.Difficulty = r.FormValue("difficulty")
		req.Visible = r.FormValue("visible") == "on"
	}

	challenge, err := h.db.CreateChallenge(req.Name, req.Description, req.Category, req.Difficulty, req.Tags, req.Visible)
	if err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to create challenge", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to create challenge`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(challenge)
	} else {
		// Return HTML card for HTMX
		w.Header().Set("Content-Type", "text/html")
		diffColor := h.getDifficultyColor(challenge.Difficulty)
		visibility := ""
		if !challenge.Visible {
			visibility = `<span class="ml-2 text-xs bg-gray-700 text-gray-300 px-2 py-1 rounded">Hidden</span>`
		}
		html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
                <div class="flex justify-between items-start mb-3">
                    <div>
                        <h3 class="text-xl font-bold text-white">%s</h3>
                        <p class="text-sm text-gray-400">
                            Category: <span class="text-blue-400">%s</span> •
                            Difficulty: <span class="font-medium %s">%s</span>
                            %s
                        </p>
                    </div>
                </div>
                <p class="text-gray-300 mb-4">%s</p>
                <div class="flex gap-2">
                    <button hx-get="/admin/challenges/%s/edit" hx-target="#challenge-%s" hx-swap="outerHTML" class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">
                        Edit
                    </button>
                    <button hx-delete="/api/admin/challenges/%s" hx-target="#challenge-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this challenge? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">
                        Delete
                    </button>
                </div>
            </div>`,
			challenge.ID,
			challenge.Name,
			challenge.Category,
			diffColor,
			challenge.Difficulty,
			visibility,
			challenge.Description,
			challenge.ID, challenge.ID,
			challenge.ID, challenge.ID,
		)
		w.Write([]byte(html))
	}
}

// getDifficultyColor returns the text color class for a difficulty name
func (h *ChallengeHandler) getDifficultyColor(name string) string {
	d, err := h.db.GetDifficultyByName(name)
	if err != nil {
		return "text-gray-400"
	}
	return d.TextColor
}

func (h *ChallengeHandler) UpdateChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	contentType := r.Header.Get("Content-Type")
	var req CreateChallengeRequest

	if strings.Contains(contentType, "application/json") {
		// JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Invalid request</div>`))
			return
		}
		req.Name = r.FormValue("name")
		req.Description = r.FormValue("description")
		// Multi-select categories: join with comma
		categories := r.Form["category"]
		req.Category = strings.Join(categories, ",")
		req.Difficulty = r.FormValue("difficulty")
		req.Visible = r.FormValue("visible") == "on"
	}

	if err := h.db.UpdateChallenge(id, req.Name, req.Description, req.Category, req.Difficulty, req.Tags, req.Visible); err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to update challenge", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to update challenge`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Challenge updated"))
	} else {
		// Return updated card for HTMX
		w.Header().Set("Content-Type", "text/html")
		challenge, _ := h.db.GetChallengeByID(id)
		diffColor := h.getDifficultyColor(challenge.Difficulty)
		visibility := ""
		if !challenge.Visible {
			visibility = `<span class="ml-2 text-xs bg-gray-700 text-gray-300 px-2 py-1 rounded">Hidden</span>`
		}
		html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
                <div class="flex justify-between items-start mb-3">
                    <div>
                        <h3 class="text-xl font-bold text-white">%s</h3>
                        <p class="text-sm text-gray-400">
                            Category: <span class="text-blue-400">%s</span> •
                            Difficulty: <span class="font-medium %s">%s</span>
                            %s
                        </p>
                    </div>
                </div>
                <p class="text-gray-300 mb-4">%s</p>
                <div class="flex gap-2">
                    <button hx-get="/admin/challenges/%s/edit" hx-target="#challenge-%s" hx-swap="outerHTML" class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">Edit</button>
                    <button hx-delete="/api/admin/challenges/%s" hx-target="#challenge-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this challenge? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">Delete</button>
                </div>
            </div>`,
			challenge.ID,
			challenge.Name,
			challenge.Category,
			diffColor,
			challenge.Difficulty,
			visibility,
			challenge.Description,
			challenge.ID, challenge.ID,
			challenge.ID, challenge.ID,
		)
		w.Write([]byte(html))
	}
}

func (h *ChallengeHandler) DeleteChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.db.DeleteChallenge(id); err != nil {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to delete challenge", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Failed to delete challenge</div>`))
		}
		return
	}

	// For HTMX, return empty response (element will be removed by hx-swap)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
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
	contentType := r.Header.Get("Content-Type")
	var req CreateQuestionRequest

	if strings.Contains(contentType, "application/json") {
		// JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`Invalid request`))
			return
		}
		req.ChallengeID = r.FormValue("challenge_id")
		req.Name = r.FormValue("name")
		req.Description = r.FormValue("description")
		req.Flag = r.FormValue("flag")
		flagMask := r.FormValue("flag_mask")
		if flagMask != "" {
			req.FlagMask = &flagMask
		}
		req.CaseSensitive = r.FormValue("case_sensitive") == "on"
		points := 100
		if p := r.FormValue("points"); p != "" {
			fmt.Sscanf(p, "%d", &points)
		}
		req.Points = points
	}

	question, err := h.db.CreateQuestion(req.ChallengeID, req.Name, req.Description, req.Flag, req.FlagMask, req.CaseSensitive, req.Points, req.FileURL)
	if err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to create question", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to create question`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(question)
	} else {
		// Return HTML card for HTMX
		w.Header().Set("Content-Type", "text/html")
		flagMaskStr := ""
		if question.FlagMask != nil {
			flagMaskStr = fmt.Sprintf(`<span class="ml-2">Mask: <code class="bg-dark-bg px-2 py-1 rounded text-yellow-400">%s</code></span>`, *question.FlagMask)
		}
		html := fmt.Sprintf(`<div id="question-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
                <div class="mb-3">
                    <h3 class="text-xl font-bold text-white">%s</h3>
                    <p class="text-sm text-gray-400">
                        Challenge: <span class="text-blue-400">%s</span> •
                        Points: <span class="text-green-400 font-medium">%d</span>
                    </p>
                </div>
                <p class="text-gray-300 mb-2 text-sm">%s</p>
                <p class="text-gray-400 text-xs mb-4">
                    Flag: <code class="bg-dark-bg px-2 py-1 rounded text-purple-400">%s</code>
                    %s
                </p>
                <div class="flex gap-2">
                    <button hx-get="/admin/questions/%s/edit" hx-target="#question-%s" hx-swap="outerHTML" class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">
                        Edit
                    </button>
                    <button hx-delete="/api/admin/questions/%s" hx-target="#question-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this question? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">
                        Delete
                    </button>
                </div>
            </div>`,
			question.ID,
			question.Name,
			question.ChallengeID,
			question.Points,
			question.Description,
			question.Flag,
			flagMaskStr,
			question.ID, question.ID,
			question.ID, question.ID,
		)
		w.Write([]byte(html))
	}
}

func (h *ChallengeHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	contentType := r.Header.Get("Content-Type")
	var req CreateQuestionRequest

	if strings.Contains(contentType, "application/json") {
		// JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Invalid request</div>`))
			return
		}
		req.ChallengeID = r.FormValue("challenge_id")
		req.Name = r.FormValue("name")
		req.Description = r.FormValue("description")
		req.Flag = r.FormValue("flag")
		flagMask := r.FormValue("flag_mask")
		if flagMask != "" {
			req.FlagMask = &flagMask
		}
		req.CaseSensitive = r.FormValue("case_sensitive") == "on"
		points := 100
		if p := r.FormValue("points"); p != "" {
			fmt.Sscanf(p, "%d", &points)
		}
		req.Points = points
	}

	if err := h.db.UpdateQuestion(id, req.Name, req.Description, req.Flag, req.FlagMask, req.CaseSensitive, req.Points, req.FileURL); err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to update question", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to update question`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Question updated"))
	} else {
		// Return updated card for HTMX
		w.Header().Set("Content-Type", "text/html")
		question, _ := h.db.GetQuestionByID(id)
		flagMaskStr := ""
		if question.FlagMask != nil {
			flagMaskStr = fmt.Sprintf(`<span class="ml-2">Mask: <code class="bg-dark-bg px-2 py-1 rounded text-yellow-400">%s</code></span>`, *question.FlagMask)
		}
		html := fmt.Sprintf(`<div id="question-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
                <div class="mb-3">
                    <h3 class="text-xl font-bold text-white">%s</h3>
                    <p class="text-sm text-gray-400">
                        Challenge: <span class="text-blue-400">%s</span> •
                        Points: <span class="text-green-400 font-medium">%d</span>
                    </p>
                </div>
                <p class="text-gray-300 mb-2 text-sm">%s</p>
                <p class="text-gray-400 text-xs mb-4">
                    Flag: <code class="bg-dark-bg px-2 py-1 rounded text-purple-400">%s</code>
                    %s
                </p>
                <div class="flex gap-2">
                    <button hx-get="/admin/questions/%s/edit" hx-target="#question-%s" hx-swap="outerHTML" class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">Edit</button>
                    <button hx-delete="/api/admin/questions/%s" hx-target="#question-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this question? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">Delete</button>
                </div>
            </div>`,
			question.ID,
			question.Name,
			question.ChallengeID,
			question.Points,
			question.Description,
			question.Flag,
			flagMaskStr,
			question.ID, question.ID,
			question.ID, question.ID,
		)
		w.Write([]byte(html))
	}
}

func (h *ChallengeHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.db.DeleteQuestion(id); err != nil {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to delete question", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to delete question`))
		}
		return
	}

	// For HTMX, return empty response (element will be removed by hx-swap)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// Hint handlers

type CreateHintRequest struct {
	QuestionID string `json:"question_id"`
	Content    string `json:"content"`
	Cost       int    `json:"cost"`
	Order      int    `json:"order"`
}

func (h *ChallengeHandler) CreateHint(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	var req CreateHintRequest

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Invalid request</div>`))
			return
		}
		req.QuestionID = r.FormValue("question_id")
		req.Content = r.FormValue("content")
		if cost := r.FormValue("cost"); cost != "" {
			fmt.Sscanf(cost, "%d", &req.Cost)
		}
		if order := r.FormValue("order"); order != "" {
			fmt.Sscanf(order, "%d", &req.Order)
		}
	}

	if req.QuestionID == "" || req.Content == "" {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Missing required fields</div>`))
		}
		return
	}

	// Check if hint with this order already exists for this question
	existingHints, _ := h.db.GetHintsByQuestionID(req.QuestionID)
	for _, h := range existingHints {
		if h.Order == req.Order {
			errMsg := "A hint with this order number already exists for this question"
			http.Error(w, errMsg, http.StatusConflict)
			return
		}
	}

	hint, err := h.db.CreateHint(req.QuestionID, req.Content, req.Cost, req.Order)
	if err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to create hint", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`<div class="text-red-400">Failed to create hint</div>`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hint)
	} else {
		// Return hint card for HTMX
		w.Header().Set("Content-Type", "text/html")
		html := fmt.Sprintf(`<div id="hint-%s" class="bg-dark-surface border border-dark-border rounded-lg p-4 mb-2">
                <p class="text-gray-300 mb-2">%s</p>
                <div class="flex justify-between items-center text-xs text-gray-400 mb-2">
                    <span>Order: %d | Cost: %d points</span>
                </div>
                <div class="flex gap-2">
                    <button class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">Edit</button>
                    <button hx-delete="/api/admin/hints/%s" hx-target="#hint-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this hint? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">Delete</button>
                </div>
            </div>`,
			hint.ID,
			hint.Content,
			hint.Order,
			hint.Cost,
			hint.ID, hint.ID,
		)
		w.Write([]byte(html))
	}
}

func (h *ChallengeHandler) UpdateHint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	contentType := r.Header.Get("Content-Type")
	var req CreateHintRequest

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		// Form data from HTMX
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400">Invalid request</div>`))
			return
		}
		req.Content = r.FormValue("content")
		if cost := r.FormValue("cost"); cost != "" {
			fmt.Sscanf(cost, "%d", &req.Cost)
		}
		if order := r.FormValue("order"); order != "" {
			fmt.Sscanf(order, "%d", &req.Order)
		}
	}

	if err := h.db.UpdateHint(id, req.Content, req.Cost, req.Order); err != nil {
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to update hint", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`<div class="text-red-400">Failed to update hint</div>`))
		}
		return
	}

	if strings.Contains(contentType, "application/json") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hint updated"))
	} else {
		// Return updated hint card for HTMX
		w.Header().Set("Content-Type", "text/html")
		hint, _ := h.db.GetHintByID(id)
		html := fmt.Sprintf(`<div id="hint-%s" class="bg-dark-surface border border-dark-border rounded-lg p-4 mb-2">
                <p class="text-gray-300 mb-2">%s</p>
                <div class="flex justify-between items-center text-xs text-gray-400 mb-2">
                    <span>Order: %d | Cost: %d points</span>
                </div>
                <div class="flex gap-2">
                    <button class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">Edit</button>
                    <button hx-delete="/api/admin/hints/%s" hx-target="#hint-%s" hx-swap="outerHTML swap:1s" hx-confirm="Delete this hint? This action cannot be undone." class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">Delete</button>
                </div>
            </div>`,
			hint.ID,
			hint.Content,
			hint.Order,
			hint.Cost,
			hint.ID, hint.ID,
		)
		w.Write([]byte(html))
	}
}

func (h *ChallengeHandler) DeleteHint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.db.DeleteHint(id); err != nil {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			http.Error(w, "Failed to delete hint", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Failed to delete hint`))
		}
		return
	}

	// For HTMX, return empty response (element will be removed by hx-swap)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// GetChallengesDropdown returns challenges as HTML options for dropdown
func (h *ChallengeHandler) GetChallengesDropdown(w http.ResponseWriter, r *http.Request) {
	challenges, err := h.db.GetChallenges(false) // false to include hidden challenges in admin
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<option value="">Select a challenge</option>`)

	for _, c := range challenges {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, c.ID, c.Name)
	}
}

// GetQuestionsDropdown returns questions with challenge names as HTML options for dropdown
func (h *ChallengeHandler) GetQuestionsDropdown(w http.ResponseWriter, r *http.Request) {
	questions, err := h.db.GetAllQuestionsWithChallenge()
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<option value="">Select a question...</option>`)

	for _, q := range questions {
		fmt.Fprintf(w, `<option value="%s">%s → %s (%d points)</option>`, q.ID, q.ChallengeName, q.Name, q.Points)
	}
}

// GetNextHintOrder returns the next order number for a question's hints
func (h *ChallengeHandler) GetNextHintOrder(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "questionId")

	nextOrder, err := h.db.GetNextHintOrder(questionID)
	if err != nil {
		http.Error(w, "Failed to get next order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"nextOrder": nextOrder})
}
