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
		http.Error(w, "You must be in a team to register", http.StatusBadRequest)
		return
	}
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.RegisterTeamForCompetition(id, *user.TeamID); err != nil {
		errMsg := err.Error()
		// Only forward known user-facing messages; hide raw DB errors
		if errMsg != "registration is closed" && errMsg != "competition has ended" {
			errMsg = "Failed to register team"
		}
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "registered"})
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

// TODO: GetScoreboardEvolution - per-competition score evolution chart data (not yet implemented)

// parseOptionalTime parses datetime-local input format or RFC3339.
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
