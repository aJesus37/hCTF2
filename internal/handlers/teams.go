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

type TeamHandler struct {
	db *database.DB
}

func NewTeamHandler(db *database.DB) *TeamHandler {
	return &TeamHandler{db: db}
}

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateTeam godoc
// @Summary Create a new team
// @Tags Teams
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param team body CreateTeamRequest true "Team data"
// @Success 200 {object} models.Team
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams [post]
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user already in a team
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID != nil {
		http.Error(w, "You are already in a team. Leave your current team first.", http.StatusBadRequest)
		return
	}

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Name == "" {
		http.Error(w, "Team name required", http.StatusBadRequest)
		return
	}

	// Create team
	team, err := h.db.CreateTeam(req.Name, req.Description, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "Team name already exists", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create team", http.StatusInternalServerError)
		}
		return
	}

	// Add creator to team
	if err := h.db.JoinTeam(claims.UserID, team.ID); err != nil {
		http.Error(w, "Failed to join team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// JoinTeam godoc
// @Summary Join a team using an invite code
// @Tags Teams
// @Produce json
// @Security CookieAuth
// @Param invite_id path string true "Team invite code"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/join/{invite_id} [post]
func (h *TeamHandler) JoinTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	inviteID := chi.URLParam(r, "invite_id")

	// Check if user already in a team
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID != nil {
		http.Error(w, "Already in a team", http.StatusBadRequest)
		return
	}

	// Look up team by invite code
	team, err := h.db.GetTeamByInviteID(inviteID)
	if err != nil {
		http.Error(w, "Invalid invite code", http.StatusNotFound)
		return
	}

	// Join team
	if err := h.db.JoinTeam(claims.UserID, team.ID); err != nil {
		http.Error(w, "Failed to join team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Joined team successfully"}`))
}

// LeaveTeam godoc
// @Summary Leave the current team
// @Description Team owners must transfer ownership or disband before leaving.
// @Tags Teams
// @Produce json
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/leave [post]
func (h *TeamHandler) LeaveTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID == nil {
		http.Error(w, "Not in a team", http.StatusBadRequest)
		return
	}

	// Check if user is team owner
	team, err := h.db.GetTeamByID(*user.TeamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	if team.OwnerID == claims.UserID {
		http.Error(w, "Team owner cannot leave. Transfer ownership or disband team.", http.StatusForbidden)
		return
	}

	// Leave team
	if err := h.db.LeaveTeam(claims.UserID); err != nil {
		http.Error(w, "Failed to leave team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Left team successfully"}`))
}

// TransferOwnership godoc
// @Summary Transfer team ownership to another member (owner only)
// @Tags Teams
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body object{new_owner_id=string} true "New owner user ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/transfer-ownership [post]
func (h *TeamHandler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID == nil {
		http.Error(w, "Not in a team", http.StatusBadRequest)
		return
	}

	// Check if user is team owner
	team, err := h.db.GetTeamByID(*user.TeamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	if team.OwnerID != claims.UserID {
		http.Error(w, "Only team owner can transfer ownership", http.StatusForbidden)
		return
	}

	// Parse request body
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	newOwnerID, ok := req["new_owner_id"]
	if !ok || newOwnerID == "" {
		http.Error(w, "New owner ID required", http.StatusBadRequest)
		return
	}

	// Verify new owner is a team member
	newOwner, err := h.db.GetUserByID(newOwnerID)
	if err != nil {
		http.Error(w, "New owner not found", http.StatusNotFound)
		return
	}

	if newOwner.TeamID == nil || *newOwner.TeamID != team.ID {
		http.Error(w, "New owner must be a team member", http.StatusBadRequest)
		return
	}

	// Transfer ownership
	if err := h.db.TransferTeamOwnership(team.ID, newOwnerID); err != nil {
		http.Error(w, "Failed to transfer ownership", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Ownership transferred successfully"}`))
}

// DisbandTeam godoc
// @Summary Disband the team (owner only)
// @Tags Teams
// @Produce json
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/disband [post]
func (h *TeamHandler) DisbandTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID == nil {
		http.Error(w, "Not in a team", http.StatusBadRequest)
		return
	}

	// Check if user is team owner
	team, err := h.db.GetTeamByID(*user.TeamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	if team.OwnerID != claims.UserID {
		http.Error(w, "Only team owner can disband the team", http.StatusForbidden)
		return
	}

	// Delete the team (CASCADE will handle removing team members)
	if err := h.db.DeleteTeam(*user.TeamID); err != nil {
		http.Error(w, "Failed to disband team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Team disbanded successfully"}`))
}

// ListTeams godoc
// @Summary List all teams
// @Description Invite codes are only visible to team members.
// @Tags Teams
// @Produce json
// @Success 200 {array} object
// @Failure 500 {object} object{error=string}
// @Router /teams [get]
func (h *TeamHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.db.GetAllTeams()
	if err != nil {
		http.Error(w, "Failed to fetch teams", http.StatusInternalServerError)
		return
	}

	// Get current user (optional)
	claims := auth.GetUserFromContext(r.Context())

	// Filter invite codes from response for non-members
	var response []interface{}
	for _, team := range teams {
		teamData := map[string]interface{}{
			"id":           team.ID,
			"name":         team.Name,
			"description":  team.Description,
			"owner_id":     team.OwnerID,
			"created_at":   team.CreatedAt,
			"updated_at":   team.UpdatedAt,
		}

		// Only include invite_id and invite_permission if user is a team member
		if claims != nil && claims.UserID != "" {
			user, err := h.db.GetUserByID(claims.UserID)
			if err == nil && user.TeamID != nil && *user.TeamID == team.ID {
				teamData["invite_id"] = team.InviteID
				teamData["invite_permission"] = team.InvitePermission
			}
		}

		response = append(response, teamData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTeam godoc
// @Summary Get a team with its members
// @Tags Teams
// @Produce json
// @Param id path string true "Team ID"
// @Success 200 {object} object{team=models.Team,members=[]object}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/{id} [get]
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	team, err := h.db.GetTeamByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	members, err := h.db.GetTeamMembers(teamID)
	if err != nil {
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"team":    team,
		"members": members,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTeamScoreboard godoc
// @Summary Get team scoreboard rankings
// @Description Returns HTML table when called from HTMX (HX-Request header), otherwise returns JSON.
// @Tags Teams
// @Produce json
// @Success 200 {array} models.ScoreboardEntry
// @Failure 500 {object} object{error=string}
// @Router /teams/scoreboard [get]
func (h *TeamHandler) GetTeamScoreboard(w http.ResponseWriter, r *http.Request) {
	scoreboard, err := h.db.GetTeamScoreboard(50)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request (return HTML) or API request (return JSON)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		// Return table body rows for HTMX to insert
		fmt.Fprint(w, `<table class="w-full">
        <thead class="bg-gray-100 dark:bg-dark-bg border-b border-gray-200 dark:border-dark-border">
            <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Rank</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Team</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Points</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Solves</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Last Solve</th>
            </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-dark-border">`)

		for _, e := range scoreboard {
			rankColor := "text-gray-500 dark:text-gray-400"
			switch e.Rank {
			case 1:
				rankColor = "text-yellow-600 dark:text-yellow-400"
			case 2:
				rankColor = "text-gray-600 dark:text-gray-300"
			case 3:
				rankColor = "text-orange-600 dark:text-orange-400"
			}

			var teamName string
			var teamID string
			if e.TeamName != nil {
				teamName = *e.TeamName
			} else if e.TeamID != nil {
				teamName = "Team " + *e.TeamID
			} else {
				teamName = "-"
			}
			if e.TeamID != nil {
				teamID = *e.TeamID
			}

			fmt.Fprintf(w, `<tr class="hover:bg-gray-100 dark:hover:bg-dark-bg transition">
                <td class="px-6 py-4 whitespace-nowrap"><span class="text-sm font-bold %s">#%d</span></td>
                <td class="px-6 py-4 whitespace-nowrap text-sm"><a href="/teams/%s/profile" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium underline decoration-blue-400/50 hover:decoration-blue-600 underline-offset-2">%s</a></td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-bold text-green-600 dark:text-green-400">%d</td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">%d</td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">%s</td>
            </tr>`,
				rankColor, e.Rank, teamID, teamName, e.Points, e.SolveCount,
				e.LastSolve.Format("Jan 02, 15:04"))
		}

		fmt.Fprint(w, `        </tbody>
    </table>`)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(scoreboard)
	}
}

// RegenerateInviteCode godoc
// @Summary Regenerate the team invite code (owner only)
// @Tags Teams
// @Produce json
// @Security CookieAuth
// @Success 200 {object} object{invite_id=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/regenerate-invite [post]
func (h *TeamHandler) RegenerateInviteCode(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID == nil {
		http.Error(w, "Not in a team", http.StatusBadRequest)
		return
	}

	// Verify user is team owner
	team, err := h.db.GetTeamByID(*user.TeamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	if team.OwnerID != claims.UserID {
		http.Error(w, "Only team owner can regenerate invite code", http.StatusForbidden)
		return
	}

	// Generate new invite code
	newCode, err := h.db.RegenerateInviteID(team.ID)
	if err != nil {
		http.Error(w, "Failed to regenerate invite code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"invite_id": newCode})
}

// UpdateInvitePermission godoc
// @Summary Update who can see the team invite code (owner only)
// @Tags Teams
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body object{permission=string} true "Permission value: 'owner_only' or 'all_members'"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /teams/invite-permission [post]
func (h *TeamHandler) UpdateInvitePermission(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.TeamID == nil {
		http.Error(w, "Not in a team", http.StatusBadRequest)
		return
	}

	// Verify user is team owner
	team, err := h.db.GetTeamByID(*user.TeamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	if team.OwnerID != claims.UserID {
		http.Error(w, "Only team owner can change invite permission", http.StatusForbidden)
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	permission, ok := req["permission"]
	if !ok || (permission != "owner_only" && permission != "all_members") {
		http.Error(w, "Invalid permission value", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateInvitePermission(team.ID, permission); err != nil {
		http.Error(w, "Failed to update permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Permission updated successfully"}`))
}

