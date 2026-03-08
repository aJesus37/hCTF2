package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
	"github.com/yourusername/hctf2/internal/models"
)

type CompetitionHandler struct {
	db *database.DB
}

func NewCompetitionHandler(db *database.DB) *CompetitionHandler {
	return &CompetitionHandler{db: db}
}

func parseCompID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// ListCompetitions godoc
// @Summary List all competitions
// @Tags Competitions
// @Produce json
// @Success 200 {array} models.Competition
// @Router /api/competitions [get]
func (h *CompetitionHandler) ListCompetitions(w http.ResponseWriter, r *http.Request) {
	comps, err := h.db.ListCompetitions()
	if err != nil {
		http.Error(w, "Failed to list competitions", http.StatusInternalServerError)
		return
	}
	if comps == nil {
		comps = []models.Competition{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comps)
}

// GetCompetition godoc
// @Summary Get competition by ID
// @Tags Competitions
// @Param id path int true "Competition ID"
// @Success 200 {object} models.Competition
// @Failure 404 {object} object{error=string}
// @Router /api/competitions/{id} [get]
func (h *CompetitionHandler) GetCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	comp, err := h.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// RegisterTeam godoc
// @Summary Register current user's team for a competition
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /api/competitions/{id}/register [post]
func (h *CompetitionHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil || user.TeamID == nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="text-sm text-red-400">You must be in a team to register. <a href="/teams" class="underline hover:text-red-300">Join or create a team</a>.</div>`)
		return
	}
	id, err := parseCompID(r)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="text-sm text-red-400">Invalid competition.</div>`)
		return
	}
	if err := h.db.RegisterTeamForCompetition(id, *user.TeamID); err != nil {
		errMsg := err.Error()
		// Only forward known user-facing messages; hide raw DB errors
		if errMsg != "registration is closed" && errMsg != "competition has ended" {
			errMsg = "Failed to register team"
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="text-sm text-red-400">%s</div>`, html.EscapeString(errMsg))
		return
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<span class="text-sm text-green-500 font-medium">&#10003; Registered</span>`)
}

// GetScoreboard godoc
// @Summary Get competition scoreboard
// @Tags Competitions
// @Param id path int true "Competition ID"
// @Success 200 {array} models.CompetitionScoreboardEntry
// @Router /api/competitions/{id}/scoreboard [get]
func (h *CompetitionHandler) GetScoreboard(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	comp, err := h.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Blackout check for non-admins
	claims := auth.GetUserFromContext(r.Context())
	isAdmin := claims != nil && claims.IsAdmin
	if comp.ScoreboardBlackout && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Scores hidden until reveal"})
		return
	}

	entries, err := h.db.GetCompetitionScoreboard(id)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []models.CompetitionScoreboardEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// CreateCompetition godoc
// @Summary Create a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Success 200 {object} models.Competition
// @Router /api/admin/competitions [post]
func (h *CompetitionHandler) CreateCompetition(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	comp, err := h.db.CreateCompetition(
		name,
		r.FormValue("description"),
		r.FormValue("rules_html"),
		parseOptionalTime(r.FormValue("start_at")),
		parseOptionalTime(r.FormValue("end_at")),
		parseOptionalTime(r.FormValue("registration_start")),
		parseOptionalTime(r.FormValue("registration_end")),
	)
	if err != nil {
		http.Error(w, "Failed to create competition", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// UpdateCompetition godoc
// @Summary Update a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id} [put]
func (h *CompetitionHandler) UpdateCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	status := r.FormValue("status")
	if status == "" {
		// Preserve existing status rather than defaulting to draft
		existing, err := h.db.GetCompetitionByID(id)
		if err != nil {
			http.Error(w, "Competition not found", http.StatusNotFound)
			return
		}
		status = existing.Status
	}
	if err := h.db.UpdateCompetition(
		id,
		r.FormValue("name"),
		r.FormValue("description"),
		r.FormValue("rules_html"),
		parseOptionalTime(r.FormValue("start_at")),
		parseOptionalTime(r.FormValue("end_at")),
		parseOptionalTime(r.FormValue("registration_start")),
		parseOptionalTime(r.FormValue("registration_end")),
		status,
	); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}
	comp, err := h.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Failed to retrieve updated competition", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// DeleteCompetition godoc
// @Summary Delete a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id} [delete]
func (h *CompetitionHandler) DeleteCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteCompetition(id); err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddChallenge godoc
// @Summary Add challenge to competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/challenges [post]
func (h *CompetitionHandler) AddChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	challengeID := r.FormValue("challenge_id")
	if challengeID == "" {
		http.Error(w, "challenge_id required", http.StatusBadRequest)
		return
	}
	if err := h.db.AddChallengeToCompetition(id, challengeID); err != nil {
		http.Error(w, "Failed to add challenge", http.StatusInternalServerError)
		return
	}
	challenges, _ := h.db.GetCompetitionChallenges(id)
	if challenges == nil {
		challenges = []models.Challenge{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenges)
}

// RemoveChallenge godoc
// @Summary Remove challenge from competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Param cid path string true "Challenge ID"
// @Router /api/admin/competitions/{id}/challenges/{cid} [delete]
func (h *CompetitionHandler) RemoveChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	cid := chi.URLParam(r, "cid")
	if err := h.db.RemoveChallengeFromCompetition(id, cid); err != nil {
		http.Error(w, "Failed to remove", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTeams godoc
// @Summary List teams registered for competition
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/teams [get]
func (h *CompetitionHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	teams, err := h.db.GetCompetitionTeams(id)
	if err != nil {
		http.Error(w, "Failed to fetch teams", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		if len(teams) == 0 {
			fmt.Fprint(w, `<p class="text-sm text-gray-400">No teams registered.</p>`)
			return
		}
		fmt.Fprint(w, `<ul class="space-y-1">`)
		for _, t := range teams {
			fmt.Fprintf(w, `<li class="text-sm text-gray-700 dark:text-gray-300">%s</li>`, html.EscapeString(t.Name))
		}
		fmt.Fprint(w, `</ul>`)
		return
	}
	if teams == nil {
		teams = []models.Team{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

// ForceStart godoc
// @Summary Force-start a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/force-start [post]
func (h *CompetitionHandler) ForceStart(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.SetCompetitionStatus(id, models.CompStatusRunning); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	// Record start_at if not already set, so the scoreboard lower-bound filter works correctly.
	h.db.SetCompetitionStartAtIfUnset(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": models.CompStatusRunning})
}

// ForceEnd godoc
// @Summary Force-end a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/force-end [post]
func (h *CompetitionHandler) ForceEnd(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.SetCompetitionStatus(id, models.CompStatusEnded); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	if err := h.db.SetCompetitionFreeze(id, true); err != nil {
		log.Printf("ForceEnd: failed to freeze competition %d: %v", id, err)
		// Continue — status is already set to ended; freeze failure is logged but non-fatal
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": models.CompStatusEnded})
}

// SetFreeze godoc
// @Summary Toggle competition scoreboard freeze (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/freeze [post]
func (h *CompetitionHandler) SetFreeze(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	frozen := r.FormValue("frozen") == "1"
	if err := h.db.SetCompetitionFreeze(id, frozen); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"frozen": frozen})
}

// SetBlackout godoc
// @Summary Toggle competition scoreboard blackout (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/blackout [post]
func (h *CompetitionHandler) SetBlackout(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	blackout := r.FormValue("blackout") == "1"
	if err := h.db.SetCompetitionBlackout(id, blackout); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"blackout": blackout})
}

// GetCompetitionScoreEvolution godoc
// @Summary Get score evolution for a competition
// @Description Returns time-series score data per team for the competition chart
// @Tags Competitions
// @Produce json
// @Param id path int true "Competition ID"
// @Success 200 {object} object{intervals=[]string,series=[]object}
// @Router /api/competitions/{id}/scoreboard/evolution [get]
func (h *CompetitionHandler) GetCompetitionScoreEvolution(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	series, err := h.db.GetCompetitionScoreEvolution(id)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch evolution"}`, http.StatusInternalServerError)
		return
	}

	response := formatCompetitionEvolutionForChart(series)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func formatCompetitionEvolutionForChart(series []database.ScoreEvolutionSeries) map[string]interface{} {
	colors := []string{"#3b82f6", "#22c55e", "#a855f7", "#f97316", "#ec4899", "#14b8a6", "#f59e0b", "#8b5cf6"}

	// Collect all unique timestamps
	timeMap := make(map[string]bool)
	for _, s := range series {
		for _, p := range s.Scores {
			timeMap[p.RecordedAt.Format("01/02 15:04")] = true
		}
	}
	var intervals []string
	for t := range timeMap {
		intervals = append(intervals, t)
	}
	// Sort chronologically (format MM/DD HH:MM sorts lexicographically)
	sortStrings(intervals)

	var chartSeries []map[string]interface{}
	for i, s := range series {
		scoreAt := make(map[string]int)
		for _, p := range s.Scores {
			key := p.RecordedAt.Format("01/02 15:04")
			scoreAt[key] = p.Score
		}
		scores := make([]int, len(intervals))
		lastScore := 0
		hasValue := false
		for j, interval := range intervals {
			if val, ok := scoreAt[interval]; ok {
				scores[j] = val
				lastScore = val
				hasValue = true
			} else if hasValue {
				scores[j] = lastScore
			}
		}
		color := colors[i%len(colors)]
		chartSeries = append(chartSeries, map[string]interface{}{
			"id":     s.UserID,
			"name":   s.Name,
			"color":  color,
			"scores": scores,
		})
	}

	return map[string]interface{}{
		"intervals": intervals,
		"series":    chartSeries,
	}
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// parseOptionalTime parses datetime-local input format or RFC3339.
// GetSubmissionFeed godoc
// @Summary Live submission feed for a competition (HTMX fragment)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Success 200 {string} string "HTML fragment"
// @Router /api/competitions/{id}/submissions [get]
func (h *CompetitionHandler) GetSubmissionFeed(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	claims := auth.GetUserFromContext(r.Context())
	isAdmin := claims != nil && claims.IsAdmin
	subs, err := h.db.GetCompetitionRecentSubmissions(id, 50, isAdmin)
	if err != nil {
		http.Error(w, "Failed to fetch submissions", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	renderSubmissionFeed(w, subs, isAdmin)
}

// GetGlobalSubmissionFeed godoc
// @Summary Live submission feed across all competitions (HTMX fragment, admin only)
// @Tags Competitions
// @Security CookieAuth
// @Success 200 {string} string "HTML fragment"
// @Router /api/competitions/submissions [get]
func (h *CompetitionHandler) GetGlobalSubmissionFeed(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	isAdmin := claims != nil && claims.IsAdmin
	subs, err := h.db.GetGlobalRecentSubmissions(100, isAdmin)
	if err != nil {
		http.Error(w, "Failed to fetch submissions", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	renderSubmissionFeed(w, subs, isAdmin)
}

func renderSubmissionFeed(w http.ResponseWriter, subs []database.CompetitionSubmission, isAdmin bool) {
	if len(subs) == 0 {
		w.Write([]byte(`<p class="text-center py-6 text-gray-500 dark:text-gray-400 text-sm">No submissions yet.</p>`))
		return
	}
	w.Write([]byte(`<div class="divide-y divide-gray-100 dark:divide-dark-border">`))
	for _, s := range subs {
		actor := html.EscapeString(s.UserName)
		if s.TeamName != "" {
			actor = html.EscapeString(s.TeamName) + " (" + html.EscapeString(s.UserName) + ")"
		}
		timeStr := s.SubmittedAt.UTC().Format("Jan 02 15:04:05")
		challLink := `/challenges/` + html.EscapeString(s.ChallengeID) + `#question-` + html.EscapeString(s.QuestionID)
		challLabel := html.EscapeString(s.ChallengeName) + ` / ` + html.EscapeString(s.QuestionName)
		challHTML := `<a href="` + challLink + `" class="text-purple-500 hover:underline">` + challLabel + `</a>`
		challHTMLMuted := `<a href="` + challLink + `" class="text-purple-400 hover:underline">` + challLabel + `</a>`

		if s.IsCorrect {
			w.Write([]byte(`<div class="flex items-start gap-3 py-2 px-1">`))
			w.Write([]byte(`<span class="text-green-500 font-bold text-sm mt-0.5">✓</span>`))
			w.Write([]byte(`<div class="flex-1 min-w-0">`))
			w.Write([]byte(`<p class="text-sm text-gray-900 dark:text-white"><span class="font-semibold">` + actor + `</span> solved ` + challHTML + `</p>`))
			if isAdmin {
				w.Write([]byte(`<p class="text-xs text-green-600 dark:text-green-400 font-mono mt-0.5">` + html.EscapeString(s.SubmittedFlag) + `</p>`))
			}
			w.Write([]byte(`<p class="text-xs text-gray-400 mt-0.5">` + timeStr + `</p>`))
			w.Write([]byte(`</div></div>`))
		} else if isAdmin {
			w.Write([]byte(`<div class="flex items-start gap-3 py-2 px-1">`))
			w.Write([]byte(`<span class="text-red-500 font-bold text-sm mt-0.5">✗</span>`))
			w.Write([]byte(`<div class="flex-1 min-w-0">`))
			w.Write([]byte(`<p class="text-sm text-gray-600 dark:text-gray-400"><span class="font-semibold text-gray-800 dark:text-gray-200">` + actor + `</span> wrong attempt on ` + challHTMLMuted + `</p>`))
			w.Write([]byte(`<p class="text-xs text-red-500 font-mono mt-0.5">` + html.EscapeString(s.SubmittedFlag) + `</p>`))
			w.Write([]byte(`<p class="text-xs text-gray-400 mt-0.5">` + timeStr + `</p>`))
			w.Write([]byte(`</div></div>`))
		}
	}
	w.Write([]byte(`</div>`))
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return &t
		}
	}
	return nil
}
