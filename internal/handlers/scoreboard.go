package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yourusername/hctf2/internal/database"
)

type ScoreboardHandler struct {
	db *database.DB
}

func NewScoreboardHandler(db *database.DB) *ScoreboardHandler {
	return &ScoreboardHandler{db: db}
}

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
