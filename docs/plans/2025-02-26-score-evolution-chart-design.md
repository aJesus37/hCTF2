# Score Evolution Chart Design

**Date**: 2026-02-26
**Author**: Claude
**Status**: Approved

## Overview

Add a beautiful line chart to the scoreboard showing top users'/teams' score evolution over time. This feature supports the learning platform use case where the CTF runs alongside educational content.

## Goals

- Visualize score progression over time for top competitors
- Help users understand the competition dynamics
- Provide engaging visual feedback for learning platform integration

## Non-Goals

- Real-time WebSocket updates (HTMX polling is sufficient)
- Individual user profile charts (scoreboard only)
- Historical data beyond configured retention period

## Architecture

### Database Schema

```sql
-- Migration: 013_score_history.up.sql
CREATE TABLE score_history (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE CASCADE, -- NULL for individual mode
    score INTEGER NOT NULL,
    solve_count INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_score_history_recorded ON score_history(recorded_at);
CREATE INDEX idx_score_history_user ON score_history(user_id, recorded_at);
CREATE INDEX idx_score_history_team ON score_history(team_id, recorded_at);
```

### Data Flow

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Background     │────▶│  score_history   │◀────│  Scoreboard     │
│  Recorder       │     │  table           │     │  API            │
│  (15 min tick)  │     └──────────────────┘     └─────────────────┘
└─────────────────┘              │                        │
                                 │                        ▼
                                 │               ┌─────────────────┐
                                 │               │  Chart.js       │
                                 │               │  (via CDN)      │
                                 │               └─────────────────┘
                                 ▼
                        ┌─────────────────┐
                        │  Auto-cleanup   │
                        │  (retention)    │
                        └─────────────────┘
```

### Configuration

Stored in `settings` table:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `score_chart_enabled` | boolean | true | Global enable/disable |
| `score_chart_top_n` | integer | 20 | Number of top users to display |
| `score_chart_interval` | integer | 15 | Recording interval in minutes |
| `score_chart_retention_days` | integer | 90 | Data retention period |

## API Design

### GET /api/scoreboard/evolution

Returns score evolution data for the chart.

**Query Parameters:**
- `mode` (string): `"individual"` or `"team"`
- `limit` (int): Override default top N (max 50)

**Response:**
```json
{
  "intervals": ["2025-02-26T10:00:00Z", "2025-02-26T10:15:00Z", "2025-02-26T10:30:00Z"],
  "series": [
    {
      "id": "user_uuid",
      "name": "Alice",
      "color": "#3b82f6",
      "scores": [100, 250, 400]
    }
  ]
}
```

## UI Design

### Scoreboard Page Changes

Add above the existing table:

```html
<div class="mb-6">
  <div class="flex justify-between items-center mb-4">
    <h2 class="text-xl font-semibold">Score Evolution</h2>
    <button @click="showChart = !showChart" class="text-sm text-blue-600">
      Toggle Chart
    </button>
  </div>
  <div x-show="showChart" class="bg-white dark:bg-dark-surface rounded-lg p-4 border">
    <canvas id="scoreChart" height="400"></canvas>
  </div>
</div>
```

### Chart Configuration

- **Type**: Line chart with smooth curves (tension: 0.4)
- **Colors**: Tailwind palette (blue-500, green-500, purple-500, orange-500, pink-500, etc.)
- **X-axis**: Time labels (HH:MM format)
- **Y-axis**: Score points
- **Legend**: Positioned at top, clickable to toggle lines
- **Tooltip**: Shows score at each point

### Admin Settings UI

Add to Settings tab:
- Toggle: "Enable Score Evolution Chart"
- Number input: "Top N Competitors to Display" (10-50)
- Number input: "Recording Interval (minutes)" (5-60)
- Number input: "Data Retention (days)" (30-365)

## Background Recorder

### Algorithm

```go
func StartScoreRecorder(db *DB, interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            recordTopScores(db)
        }
    }()
}

func recordTopScores(db *DB) {
    // Get current top N users
    topUsers := db.GetScoreboard(topN)
    
    // Record each user's current score
    for _, user := range topUsers {
        db.InsertScoreHistory(user.ID, nil, user.Score, user.SolveCount)
    }
}
```

### Cleanup Job

Run daily to remove old records beyond retention period.

## Implementation Phases

1. **Database**: Migration for score_history table
2. **Backend**: Background recorder + API endpoint
3. **Frontend**: Chart.js integration + settings UI
4. **Admin**: Settings configuration panel

## Performance Considerations

- Index on (user_id, recorded_at) for fast queries
- Limit chart to 50 users max
- Data points limited by retention (90 days × 96 points/day = ~8,640 max points per user)
- Background recorder runs asynchronously, no user impact

## Security

- API endpoint is public (same as scoreboard)
- No sensitive data exposed (just scores and usernames)
- Rate limit API to prevent abuse

## Future Enhancements

- Individual user profile charts
- Export chart as image
- Compare specific users
- Zoom/pan on time axis
