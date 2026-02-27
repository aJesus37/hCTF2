package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/yourusername/hctf2/internal/database"
)

type ScoreboardHandler struct {
	db       *database.DB
	recorder ScoreRecorder
}

// ScoreRecorder interface for the background recorder
type ScoreRecorder interface {
	ForceRecord() error
	RecordUser(userID string)
}

func NewScoreboardHandler(db *database.DB, recorder ScoreRecorder) *ScoreboardHandler {
	return &ScoreboardHandler{db: db, recorder: recorder}
}

// GetScoreboard godoc
// @Summary Get individual user scoreboard rankings
// @Description Returns HTML table when called from HTMX (HX-Request header), otherwise returns JSON. Top 100 users.
// @Tags Scoreboard
// @Produce json
// @Success 200 {array} models.ScoreboardEntry
// @Failure 500 {object} object{error=string}
// @Router /scoreboard [get]
func (h *ScoreboardHandler) GetScoreboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.db.GetScoreboard(100) // Top 100
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
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">User</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Team</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Points</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Solves</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Last Solve</th>
            </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-dark-border">`)

		for _, e := range entries {
			rankColor := "text-gray-500 dark:text-gray-400"
			switch e.Rank {
			case 1:
				rankColor = "text-yellow-600 dark:text-yellow-400"
			case 2:
				rankColor = "text-gray-600 dark:text-gray-300"
			case 3:
				rankColor = "text-orange-600 dark:text-orange-400"
			}

			teamName := "-"
			if e.TeamName != nil {
				teamName = *e.TeamName
			}
			
			var teamCell string
			if e.TeamID != nil && *e.TeamID != "" {
				teamCell = fmt.Sprintf(`<a href="/teams/%s/profile" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium underline decoration-blue-400/50 hover:decoration-blue-600 underline-offset-2">%s</a>`, *e.TeamID, teamName)
			} else {
				teamCell = fmt.Sprintf(`<span class="text-gray-500 dark:text-gray-400">%s</span>`, teamName)
			}

			fmt.Fprintf(w, `<tr class="hover:bg-gray-100 dark:hover:bg-dark-bg transition">
                <td class="px-6 py-4 whitespace-nowrap"><span class="text-sm font-bold %s">#%d</span></td>
                <td class="px-6 py-4 whitespace-nowrap text-sm"><a href="/users/%s" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium underline decoration-blue-400/50 hover:decoration-blue-600 underline-offset-2">%s</a></td>
                <td class="px-6 py-4 whitespace-nowrap text-sm">%s</td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-bold text-green-600 dark:text-green-400">%d</td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">%d</td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">%s</td>
            </tr>`,
				rankColor, e.Rank, e.UserID, e.UserName, teamCell, e.Points, e.SolveCount,
				e.LastSolve.Format("Jan 02, 15:04"))
		}

		fmt.Fprint(w, `        </tbody>
    </table>`)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

// GetTeamScoreboard returns team rankings as HTML for HTMX or JSON for API
func (h *ScoreboardHandler) GetTeamScoreboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.db.GetTeamScoreboard(100) // Top 100
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

		for _, e := range entries {
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
		json.NewEncoder(w).Encode(entries)
	}
}

// CTFtimeExport godoc
// @Summary Export scoreboard in CTFtime.org JSON format
// @Description Returns team standings in the format expected by CTFtime.org. No authentication required.
// @Tags Scoreboard
// @Produce json
// @Success 200 {object} object{standings=[]object{pos=int,team=string,score=int}}
// @Failure 404 {string} string "No team data available"
// @Failure 500 {string} string "Internal server error"
// @Router /api/ctftime [get]
func (h *ScoreboardHandler) CTFtimeExport(w http.ResponseWriter, r *http.Request) {
	entries, err := h.db.GetTeamScoreboard(500)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(entries) == 0 {
		http.Error(w, "No team data available", http.StatusNotFound)
		return
	}

	type standing struct {
		Pos   int    `json:"pos"`
		Team  string `json:"team"`
		Score int    `json:"score"`
	}
	type ctftimeResponse struct {
		Standings []standing `json:"standings"`
	}

	resp := ctftimeResponse{}
	for _, e := range entries {
		name := e.UserName
		if e.TeamName != nil {
			name = *e.TeamName
		}
		resp.Standings = append(resp.Standings, standing{
			Pos:   e.Rank,
			Team:  name,
			Score: e.Points,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetScoreEvolution godoc
// @Summary Get score evolution over time for chart
// @Description Returns time-series score data for top N users. Used by Chart.js.
// @Tags Scoreboard
// @Produce json
// @Param mode query string false "Score mode: individual or team" default(individual)
// @Param limit query int false "Number of top users to include" default(20)
// @Success 200 {object} object{intervals=[]string,series=[]object}
// @Router /api/scoreboard/evolution [get]
func (h *ScoreboardHandler) GetScoreEvolution(w http.ResponseWriter, r *http.Request) {
	// Parse params
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	// Get data for last 7 days
	since := time.Now().Add(-7 * 24 * time.Hour)

	series, err := h.db.GetScoreEvolution(limit, since)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch evolution"}`, http.StatusInternalServerError)
		return
	}

	// Format response for Chart.js
	response := formatEvolutionForChart(series)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ForceScoreRecord godoc
// @Summary Manually trigger score recording (admin only)
// @Description Forces the background score recorder to capture a snapshot immediately. Admin only.
// @Tags Scoreboard
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Failure 500 {object} object{error=string}
// @Router /api/admin/scoreboard/force-record [post]
func (h *ScoreboardHandler) ForceScoreRecord(w http.ResponseWriter, r *http.Request) {
	if h.recorder == nil {
		http.Error(w, `{"error":"score recorder not initialized"}`, http.StatusInternalServerError)
		return
	}
	
	if err := h.recorder.ForceRecord(); err != nil {
		http.Error(w, `{"error":"failed to trigger recording"}`, http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Score recording triggered successfully"})
}

func formatEvolutionForChart(series []database.ScoreEvolutionSeries) map[string]interface{} {
	// Collect all unique timestamps (second-level granularity)
	timeMap := make(map[string]bool)
	for _, s := range series {
		for _, p := range s.Scores {
			timeMap[p.RecordedAt.Format("15:04:05")] = true
		}
	}

	// Sort timestamps
	var intervals []string
	for t := range timeMap {
		intervals = append(intervals, t)
	}
	sort.Strings(intervals)

	// Build series data
	colors := []string{"#3b82f6", "#22c55e", "#a855f7", "#f97316", "#ec4899", "#14b8a6", "#f59e0b", "#8b5cf6"}

	var chartSeries []map[string]interface{}
	for i, s := range series {
		// Build a map of interval -> latest score for this user
		scoreAt := make(map[string]int)
		for _, p := range s.Scores {
			key := p.RecordedAt.Format("15:04:05")
			scoreAt[key] = p.Score // last write wins (latest score at that second)
		}

		scores := make([]int, len(intervals))
		hasValue := false
		lastScore := 0
		for j, interval := range intervals {
			if val, ok := scoreAt[interval]; ok {
				scores[j] = val
				lastScore = val
				hasValue = true
			} else if hasValue {
				// Carry forward previous score
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
