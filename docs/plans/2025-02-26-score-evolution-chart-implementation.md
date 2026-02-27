# Score Evolution Chart Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Chart.js line chart to the scoreboard showing top users' score evolution over time.

**Architecture:** Background goroutine records top N scores every 15 minutes to a `score_history` table. API endpoint serves time-series data. Scoreboard page renders Chart.js visualization.

**Tech Stack:** Go, SQLite, Chart.js (CDN), HTMX, Alpine.js

---

### Task 1: Create Database Migration

**Files:**
- Create: `internal/database/migrations/013_score_history.up.sql`
- Create: `internal/database/migrations/013_score_history.down.sql`

**Step 1: Create migration file**

```sql
-- internal/database/migrations/013_score_history.up.sql
CREATE TABLE score_history (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE CASCADE,
    score INTEGER NOT NULL,
    solve_count INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_score_history_recorded ON score_history(recorded_at);
CREATE INDEX idx_score_history_user ON score_history(user_id, recorded_at);
CREATE INDEX idx_score_history_team ON score_history(team_id, recorded_at);
```

**Step 2: Create down migration**

```sql
-- internal/database/migrations/013_score_history.down.sql
DROP INDEX IF EXISTS idx_score_history_team;
DROP INDEX IF EXISTS idx_score_history_user;
DROP INDEX IF EXISTS idx_score_history_recorded;
DROP TABLE IF EXISTS score_history;
```

**Step 3: Verify migration syntax**

Run: `sqlite3 /tmp/test.db < internal/database/migrations/013_score_history.up.sql`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/database/migrations/013_score_history.*
git commit -m "feat(scoreboard): add score_history table migration"
```

---

### Task 2: Add Database Queries

**Files:**
- Modify: `internal/database/queries.go`

**Step 1: Add InsertScoreHistory method**

Add to `internal/database/queries.go` after existing methods:

```go
// InsertScoreHistory records a user's score snapshot
func (db *DB) InsertScoreHistory(userID, teamID string, score, solveCount int) error {
    query := `INSERT INTO score_history (id, user_id, team_id, score, solve_count) VALUES (?, ?, ?, ?, ?)`
    _, err := db.Exec(query, generateID(), userID, teamID, score, solveCount)
    return err
}

// ScoreEvolutionPoint represents a single data point for the chart
type ScoreEvolutionPoint struct {
    RecordedAt time.Time `json:"recorded_at"`
    Score      int       `json:"score"`
}

// ScoreEvolutionSeries represents one user's score over time
type ScoreEvolutionSeries struct {
    UserID  string                `json:"id"`
    Name    string                `json:"name"`
    Scores  []ScoreEvolutionPoint `json:"scores"`
}

// GetScoreEvolution returns score history for top N users
func (db *DB) GetScoreEvolution(limit int, since time.Time) ([]ScoreEvolutionSeries, error) {
    // Get top N users by current score
    topUsersQuery := `
        SELECT u.id, u.username 
        FROM users u
        JOIN (
            SELECT user_id, SUM(points_earned) as total
            FROM submissions s
            JOIN questions q ON s.question_id = q.id
            WHERE s.is_correct = 1 AND u.is_admin = 0
            GROUP BY user_id
            ORDER BY total DESC
            LIMIT ?
        ) scores ON u.id = scores.user_id
    `
    
    rows, err := db.Query(topUsersQuery, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var userIDs []string
    var userNames []string
    for rows.Next() {
        var id, name string
        if err := rows.Scan(&id, &name); err != nil {
            return nil, err
        }
        userIDs = append(userIDs, id)
        userNames = append(userNames, name)
    }
    
    var result []ScoreEvolutionSeries
    for i, userID := range userIDs {
        historyQuery := `
            SELECT recorded_at, score 
            FROM score_history 
            WHERE user_id = ? AND recorded_at >= ?
            ORDER BY recorded_at ASC
        `
        histRows, err := db.Query(historyQuery, userID, since)
        if err != nil {
            return nil, err
        }
        
        var points []ScoreEvolutionPoint
        for histRows.Next() {
            var p ScoreEvolutionPoint
            if err := histRows.Scan(&p.RecordedAt, &p.Score); err != nil {
                histRows.Close()
                return nil, err
            }
            points = append(points, p)
        }
        histRows.Close()
        
        result = append(result, ScoreEvolutionSeries{
            UserID: userID,
            Name:   userNames[i],
            Scores: points,
        })
    }
    
    return result, nil
}

// CleanupScoreHistory removes old records beyond retention period
func (db *DB) CleanupScoreHistory(retentionDays int) error {
    query := `DELETE FROM score_history WHERE recorded_at < datetime('now', '-? days')`
    _, err := db.Exec(query, retentionDays)
    return err
}
```

**Step 2: Add generateID helper if not exists**

Check if `generateID()` exists in queries.go. If not, add:

```go
func generateID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}
```

**Step 3: Commit**

```bash
git add internal/database/queries.go
git commit -m "feat(scoreboard): add score_history database queries"
```

---

### Task 3: Create Background Score Recorder

**Files:**
- Create: `internal/scorerecorder/recorder.go`

**Step 1: Create recorder package**

```go
// internal/scorerecorder/recorder.go
package scorerecorder

import (
    "context"
    "log"
    "time"

    "github.com/yourusername/hctf2/internal/database"
)

// Recorder periodically records score snapshots
type Recorder struct {
    db         *database.DB
    interval   time.Duration
    topN       int
    cancelFunc context.CancelFunc
}

// New creates a new score recorder
func New(db *database.DB, interval time.Duration, topN int) *Recorder {
    return &Recorder{
        db:       db,
        interval: interval,
        topN:     topN,
    }
}

// Start begins the background recording loop
func (r *Recorder) Start() {
    ctx, cancel := context.WithCancel(context.Background())
    r.cancelFunc = cancel
    
    // Do initial recording immediately
    r.record(ctx)
    
    ticker := time.NewTicker(r.interval)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                r.record(ctx)
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Start cleanup job (daily)
    go r.cleanupLoop(ctx)
    
    log.Printf("[Recorder] Started with interval=%v, topN=%d", r.interval, r.topN)
}

// Stop halts the recorder
func (r *Recorder) Stop() {
    if r.cancelFunc != nil {
        r.cancelFunc()
    }
}

func (r *Recorder) record(ctx context.Context) {
    entries, err := r.db.GetScoreboard(r.topN)
    if err != nil {
        log.Printf("[Recorder] Failed to get scoreboard: %v", err)
        return
    }
    
    for _, entry := range entries {
        teamID := ""
        if entry.TeamID != nil {
            teamID = *entry.TeamID
        }
        if err := r.db.InsertScoreHistory(entry.UserID, teamID, entry.Points, entry.SolveCount); err != nil {
            log.Printf("[Recorder] Failed to record score for user %s: %v", entry.UserID, err)
        }
    }
    
    log.Printf("[Recorder] Recorded scores for %d users", len(entries))
}

func (r *Recorder) cleanupLoop(ctx context.Context) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := r.db.CleanupScoreHistory(90); err != nil {
                log.Printf("[Recorder] Cleanup failed: %v", err)
            } else {
                log.Printf("[Recorder] Cleaned up old score history")
            }
        case <-ctx.Done():
            return
        }
    }
}
```

**Step 2: Commit**

```bash
git add internal/scorerecorder/recorder.go
git commit -m "feat(scoreboard): add background score recorder"
```

---

### Task 4: Add API Endpoint

**Files:**
- Modify: `internal/handlers/scoreboard.go`

**Step 1: Add Evolution endpoint to handler**

Add to `internal/handlers/scoreboard.go`:

```go
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

func formatEvolutionForChart(series []database.ScoreEvolutionSeries) map[string]interface{} {
    // Collect all timestamps
    timeMap := make(map[string]bool)
    for _, s := range series {
        for _, p := range s.Scores {
            timeMap[p.RecordedAt.Format("15:04")] = true
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
        scores := make([]int, len(intervals))
        // Fill in scores (simplified - assumes regular intervals)
        for j, interval := range intervals {
            for _, p := range s.Scores {
                if p.RecordedAt.Format("15:04") == interval {
                    scores[j] = p.Score
                    break
                }
            }
            // Carry forward previous score if no data point
            if j > 0 && scores[j] == 0 {
                scores[j] = scores[j-1]
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
```

**Step 2: Add imports**

Add to imports in scoreboard.go:
```go
import (
    "encoding/json"
    "fmt"
    "net/http"
    "sort"
    "strconv"
    "time"

    "github.com/yourusername/hctf2/internal/database"
)
```

**Step 3: Register route in main.go**

Add to main.go router setup (around line 420):
```go
r.Get("/api/scoreboard/evolution", scoreboardH.GetScoreEvolution)
```

**Step 4: Test endpoint**

Build and run, then:
```bash
curl http://localhost:8090/api/scoreboard/evolution
```

Expected: JSON with intervals and series arrays

**Step 5: Commit**

```bash
git add internal/handlers/scoreboard.go main.go
git commit -m "feat(scoreboard): add score evolution API endpoint"
```

---

### Task 5: Update Scoreboard Template with Chart

**Files:**
- Modify: `internal/views/templates/scoreboard.html`
- Modify: `internal/views/templates/base.html`

**Step 1: Add Chart.js CDN to base.html**

Add before closing `</head>` in base.html:
```html
{{if .ShowChart}}
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
{{end}}
```

**Step 2: Modify scoreboard.html**

Replace entire scoreboard-content block:

```html
{{define "scoreboard-content"}}
<div x-data="{ 
    mode: 'individual',
    showChart: true,
    chartInstance: null,
    init() {
        this.$watch('showChart', value => {
            if (value) this.$nextTick(() => this.renderChart())
        })
        this.$nextTick(() => this.renderChart())
    },
    renderChart() {
        if (!this.showChart) return
        const ctx = document.getElementById('scoreChart')
        if (!ctx) return
        
        fetch('/api/scoreboard/evolution?mode=' + this.mode)
            .then(r => r.json())
            .then(data => {
                if (this.chartInstance) {
                    this.chartInstance.destroy()
                }
                
                const isDark = document.documentElement.classList.contains('dark')
                const gridColor = isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'
                const textColor = isDark ? '#e2e8f0' : '#374151'
                
                const datasets = data.series.map(s => ({
                    label: s.name,
                    data: s.scores,
                    borderColor: s.color,
                    backgroundColor: s.color + '20',
                    tension: 0.4,
                    fill: false,
                    pointRadius: 3,
                    pointHoverRadius: 6
                }))
                
                this.chartInstance = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: data.intervals,
                        datasets: datasets
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        interaction: {
                            intersect: false,
                            mode: 'index'
                        },
                        plugins: {
                            legend: {
                                position: 'top',
                                labels: { color: textColor }
                            },
                            tooltip: {
                                backgroundColor: isDark ? '#1e293b' : '#ffffff',
                                titleColor: textColor,
                                bodyColor: textColor,
                                borderColor: gridColor,
                                borderWidth: 1
                            }
                        },
                        scales: {
                            x: {
                                grid: { color: gridColor },
                                ticks: { color: textColor }
                            },
                            y: {
                                grid: { color: gridColor },
                                ticks: { color: textColor },
                                beginAtZero: true
                            }
                        }
                    }
                })
            })
    }
}">
    <div class="mb-8">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-4xl font-bold text-blue-400">Scoreboard</h1>
            <div class="flex gap-2">
                <button @click="mode = 'individual'; $nextTick(() => renderChart())"
                        :class="mode === 'individual' ? 'bg-blue-600' : 'bg-gray-400 dark:bg-gray-700'"
                        class="px-4 py-2 rounded text-white font-semibold text-sm transition">
                    Individual
                </button>
                <button @click="mode = 'team'; $nextTick(() => renderChart())"
                        :class="mode === 'team' ? 'bg-blue-600' : 'bg-gray-400 dark:bg-gray-700'"
                        class="px-4 py-2 rounded text-white font-semibold text-sm transition">
                    Team
                </button>
            </div>
        </div>
        <p class="text-gray-600 dark:text-gray-300">Top ranked by points and solve time</p>
    </div>

    <!-- Score Evolution Chart -->
    <div class="mb-6 bg-white dark:bg-dark-surface border border-gray-200 dark:border-dark-border rounded-lg p-4">
        <div class="flex justify-between items-center mb-4">
            <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-200">Score Evolution</h2>
            <button @click="showChart = !showChart" 
                    class="text-sm text-blue-600 dark:text-blue-400 hover:underline">
                <span x-text="showChart ? 'Hide Chart' : 'Show Chart'"></span>
            </button>
        </div>
        <div x-show="showChart" class="relative" style="height: 400px;">
            <canvas id="scoreChart"></canvas>
        </div>
    </div>

    <!-- Individual Scoreboard -->
    <div id="individual-view" x-show="mode === 'individual'" ...existing table code...>
    
    <!-- Team Scoreboard -->
    <div id="team-view" x-show="mode === 'team'" ...existing table code...>
</div>
{{end}}
```

**Step 3: Update base.html to pass ShowChart flag**

Find where scoreboard is rendered in main.go and add ShowChart: true to data.

**Step 4: Commit**

```bash
git add internal/views/templates/scoreboard.html internal/views/templates/base.html
git commit -m "feat(scoreboard): add Chart.js evolution chart to UI"
```

---

### Task 6: Integrate Recorder in Main

**Files:**
- Modify: `main.go`

**Step 1: Initialize recorder on startup**

Add to Server struct (around line 103):
```go
scoreRecorder    *scorerecorder.Recorder
```

Add to main() after db initialization (around line 290):
```go
// Initialize score recorder
recorder := scorerecorder.New(db, 15*time.Minute, 20)
recorder.Start()
defer recorder.Stop()
```

**Step 2: Add import**

```go
import "github.com/yourusername/hctf2/internal/scorerecorder"
```

**Step 3: Test**

Build and run. Check logs for "[Recorder] Started" message.

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat(scoreboard): integrate score recorder in main"
```

---

### Task 7: Add Admin Settings

**Files:**
- Modify: `internal/handlers/settings.go`
- Modify: `internal/views/templates/admin.html`

**Step 1: Add settings to GetSettings/UpdateSettings**

Add to settings handler:
```go
// ScoreChartEnabled returns whether score chart is enabled
func (db *DB) GetScoreChartEnabled() bool {
    // Default to true
    return true
}
```

**Step 2: Add UI to admin settings tab**

Add to admin.html settings tab:
```html
<div class="mb-6">
    <h3 class="text-lg font-semibold mb-3">Scoreboard Chart</h3>
    <div class="space-y-4">
        <label class="flex items-center">
            <input type="checkbox" name="score_chart_enabled" class="mr-2" {{if .ScoreChartEnabled}}checked{{end}}>
            <span>Enable score evolution chart</span>
        </label>
        <div>
            <label class="block text-sm font-medium mb-1">Top N Competitors</label>
            <input type="number" name="score_chart_top_n" value="{{.ScoreChartTopN}}" 
                   min="10" max="50" class="border rounded px-3 py-2 w-32">
        </div>
    </div>
</div>
```

**Step 3: Commit**

```bash
git add internal/handlers/settings.go internal/views/templates/admin.html
git commit -m "feat(scoreboard): add chart settings to admin panel"
```

---

### Task 8: Final Testing

**Step 1: Run tests**
```bash
task test
```

**Step 2: Manual test**
```bash
task rebuild
./hctf2 --dev --admin-email admin@test.com --admin-password test
```

Navigate to scoreboard, verify:
- Chart loads with data
- Toggle button works
- Mode switch updates chart
- Dark mode styling is correct

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix(scoreboard): address chart rendering issues"
```

---

## Summary

This implementation adds:
1. `score_history` table with indexes
2. Background recorder (15 min intervals)
3. `/api/scoreboard/evolution` endpoint
4. Chart.js visualization on scoreboard page
5. Admin settings for configuration

**Files touched:**
- `internal/database/migrations/013_score_history.*`
- `internal/database/queries.go`
- `internal/scorerecorder/recorder.go` (new)
- `internal/handlers/scoreboard.go`
- `internal/views/templates/scoreboard.html`
- `internal/views/templates/base.html`
- `main.go`
