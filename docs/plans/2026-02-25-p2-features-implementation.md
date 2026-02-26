# P2 Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 7 remaining P2 open source readiness features: rate limiting, score freezing, CTFtime export, file attachments, challenge import/export, and two accessibility improvements.

**Architecture:** Sequential by priority. Each task is self-contained with its own migration, handler, and UI change. All frontend changes must be validated with agent-browser before marking complete.

**Tech Stack:** Go 1.24, Chi router, SQLite (modernc.org/sqlite), HTMX, Alpine.js, Tailwind CSS (self-hosted), `golang.org/x/time/rate`, `aws-sdk-go-v2` (optional, S3 only).

**Validation workflow:**
```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
npx agent-browser --session hctf2 open http://localhost:8092/login && \
npx agent-browser --session hctf2 fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2 fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2 find role button click --name Login
```

---

## Task 1: Rate Limiting on Flag Submissions

**Files:**
- Create: `internal/ratelimit/ratelimit.go`
- Create: `internal/ratelimit/ratelimit_test.go`
- Modify: `main.go` (add flag + wire limiter into SubmitFlag handler)
- Modify: `internal/handlers/challenges.go` (accept limiter, check before submit)

**Step 1: Write the failing test**

Create `internal/ratelimit/ratelimit_test.go`:
```go
package ratelimit_test

import (
	"testing"
	"time"

	"github.com/yourusername/hctf2/internal/ratelimit"
)

func TestLimiter_AllowsUnderLimit(t *testing.T) {
	l := ratelimit.New(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !l.Allow("user-1") {
			t.Fatalf("expected Allow() = true on attempt %d", i+1)
		}
	}
}

func TestLimiter_BlocksOverLimit(t *testing.T) {
	l := ratelimit.New(2, time.Minute)
	l.Allow("user-1")
	l.Allow("user-1")
	if l.Allow("user-1") {
		t.Fatal("expected Allow() = false after exceeding limit")
	}
}

func TestLimiter_IsolatesUsers(t *testing.T) {
	l := ratelimit.New(1, time.Minute)
	l.Allow("user-1")
	if !l.Allow("user-2") {
		t.Fatal("user-2 should not be affected by user-1's limit")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/ratelimit/... -v
```
Expected: FAIL with "no Go files" or package not found.

**Step 3: Implement the rate limiter**

Create `internal/ratelimit/ratelimit.go`:
```go
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter is a per-user token bucket rate limiter.
type Limiter struct {
	mu       sync.Mutex
	limiters map[string]*entry
	rate     rate.Limit
	burst    int
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New creates a Limiter allowing `requests` per `window` per user.
func New(requests int, window time.Duration) *Limiter {
	l := &Limiter{
		limiters: make(map[string]*entry),
		rate:     rate.Every(window / time.Duration(requests)),
		burst:    requests,
	}
	go l.cleanup()
	return l
}

// Allow returns true if the user is within their rate limit.
func (l *Limiter) Allow(userID string) bool {
	l.mu.Lock()
	e, ok := l.limiters[userID]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.limiters[userID] = e
	}
	e.lastSeen = time.Now()
	l.mu.Unlock()
	return e.limiter.Allow()
}

// cleanup removes idle entries every 5 minutes.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for id, e := range l.limiters {
			if time.Since(e.lastSeen) > 10*time.Minute {
				delete(l.limiters, id)
			}
		}
		l.mu.Unlock()
	}
}
```

**Step 4: Add golang.org/x/time/rate dependency**

```bash
go get golang.org/x/time/rate
go mod tidy
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/ratelimit/... -v
```
Expected: PASS all 3 tests.

**Step 6: Wire limiter into the server**

In `main.go`, add CLI flag near other flags (around line 180):
```go
submissionRateLimit = flag.Int("submission-rate-limit", 5, "Max flag submissions per minute per user (0 = unlimited)")
```

In the `server` struct (search for `type server struct`), add:
```go
submitLimiter *ratelimit.Limiter
```

In the server initialization (where handlers are created), add after existing handler setup:
```go
if *submissionRateLimit > 0 {
    s.submitLimiter = ratelimit.New(*submissionRateLimit, time.Minute)
}
```

Add import: `"github.com/yourusername/hctf2/internal/ratelimit"`

**Step 7: Apply limiter in SubmitFlag handler**

In `internal/handlers/challenges.go`, find `SubmitFlag` function. Add at the top of the handler, after extracting the user from context:

First, update `ChallengeHandler` struct to accept the limiter:
```go
type ChallengeHandler struct {
    db            *database.DB
    submitLimiter *ratelimit.Limiter // nil = unlimited
}
```

Update constructor `NewChallengeHandler` to accept it:
```go
func NewChallengeHandler(db *database.DB, limiter *ratelimit.Limiter) *ChallengeHandler {
    return &ChallengeHandler{db: db, submitLimiter: limiter}
}
```

In `SubmitFlag`, after getting `userID` from context, add:
```go
if h.submitLimiter != nil && !h.submitLimiter.Allow(userID) {
    w.Header().Set("Content-Type", "text/html")
    w.WriteHeader(http.StatusTooManyRequests)
    w.Write([]byte(`<div class="p-3 bg-red-900/50 border border-red-700 rounded text-red-300 text-sm">Too many attempts. Please wait before trying again.</div>`))
    return
}
```

In `main.go`, update the `NewChallengeHandler` call to pass the limiter:
```go
s.challengeH = handlers.NewChallengeHandler(db, s.submitLimiter)
```

**Step 8: Build and smoke test**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
# Submit a flag 6 times quickly via curl to verify 429 on 6th attempt
```

**Step 9: Commit**

```bash
git add internal/ratelimit/ internal/handlers/challenges.go main.go go.mod go.sum
git commit -m "feat(security): add per-user rate limiting on flag submissions

Limits flag submission attempts to 5/min per user by default.
Configurable via --submission-rate-limit flag (0 = unlimited).
Returns HTTP 429 with HTMX-friendly error fragment."
```

---

## Task 2: Score Freezing

**Files:**
- Create: `internal/database/migrations/010_score_freeze.up.sql`
- Create: `internal/database/migrations/010_score_freeze.down.sql`
- Modify: `internal/database/queries.go` (freeze-aware scoreboard queries, freeze settings getters/setters)
- Modify: `internal/handlers/scoreboard.go` (pass freeze params)
- Modify: `internal/handlers/settings.go` (freeze admin endpoint)
- Modify: `main.go` (register freeze route)
- Modify: `internal/views/templates/admin.html` (freeze UI in Settings tab)

**Step 1: Create migrations**

`internal/database/migrations/010_score_freeze.up.sql`:
```sql
INSERT OR IGNORE INTO site_settings (key, value) VALUES ('freeze_enabled', '0');
INSERT OR IGNORE INTO site_settings (key, value) VALUES ('freeze_at', '');
```

`internal/database/migrations/010_score_freeze.down.sql`:
```sql
DELETE FROM site_settings WHERE key IN ('freeze_enabled', 'freeze_at');
```

**Step 2: Add freeze queries to queries.go**

In `internal/database/queries.go`, add these helper functions:

```go
// GetScoreFreeze returns whether the scoreboard is frozen and when.
func (db *DB) GetScoreFreeze() (enabled bool, freezeAt *time.Time, err error) {
    enabledStr, _ := db.GetSetting("freeze_enabled")
    enabled = enabledStr == "1"

    freezeAtStr, _ := db.GetSetting("freeze_at")
    if freezeAtStr != "" {
        t, err := time.Parse(time.RFC3339, freezeAtStr)
        if err == nil {
            freezeAt = &t
        }
    }
    return enabled, freezeAt, nil
}

// SetScoreFreeze sets the freeze state.
func (db *DB) SetScoreFreeze(enabled bool, freezeAt *time.Time) error {
    enabledVal := "0"
    if enabled {
        enabledVal = "1"
    }
    if err := db.SetSetting("freeze_enabled", enabledVal); err != nil {
        return err
    }
    freezeAtVal := ""
    if freezeAt != nil {
        freezeAtVal = freezeAt.UTC().Format(time.RFC3339)
    }
    return db.SetSetting("freeze_at", freezeAtVal)
}

// IsFrozen returns true if the scoreboard is currently frozen.
func (db *DB) IsFrozen() bool {
    enabled, freezeAt, err := db.GetScoreFreeze()
    if err != nil || !enabled {
        return false
    }
    if freezeAt == nil {
        return true // enabled with no time = frozen immediately
    }
    return time.Now().After(*freezeAt)
}
```

**Step 3: Add freeze filter to scoreboard queries**

In `GetScoreboard`, replace the `LEFT JOIN submissions` line to add the freeze condition. Find the query string and modify the submissions join:

```sql
LEFT JOIN submissions s ON u.id = s.user_id AND s.is_correct = 1
    AND (:freeze_at = '' OR s.created_at <= :freeze_at)
```

Update the function signature:
```go
func (db *DB) GetScoreboard(limit int) ([]models.ScoreboardEntry, error) {
    freezeAt := ""
    if db.IsFrozen() {
        _, ft, _ := db.GetScoreFreeze()
        if ft != nil {
            freezeAt = ft.UTC().Format("2006-01-02 15:04:05")
        } else {
            freezeAt = time.Now().UTC().Format("2006-01-02 15:04:05")
        }
    }
    // use named param :freeze_at in query
```

Do the same for `GetTeamScoreboard`.

Note: modernc.org/sqlite supports named params. Alternatively, use a Go-level time check by passing `freezeAt` as a query parameter string.

**Step 4: Add freeze admin handler**

In `internal/handlers/settings.go`, add:

```go
// SetScoreFreeze handles POST /api/admin/settings/freeze
func (h *SettingsHandler) SetScoreFreeze(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseForm(); err != nil {
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    enabled := r.FormValue("freeze_enabled") == "1"
    freezeAtStr := r.FormValue("freeze_at")

    var freezeAt *time.Time
    if freezeAtStr != "" {
        t, err := time.Parse("2006-01-02T15:04", freezeAtStr)
        if err == nil {
            ft := t.UTC()
            freezeAt = &ft
        }
    }

    if err := h.db.SetScoreFreeze(enabled, freezeAt); err != nil {
        http.Error(w, "Failed to save", http.StatusInternalServerError)
        return
    }

    frozen := h.db.IsFrozen()
    statusText := "Live"
    statusClass := "text-green-400"
    if frozen {
        statusText = "Frozen"
        statusClass = "text-blue-400"
    }

    w.Header().Set("Content-Type", "text/html")
    fmt.Fprintf(w, `<span id="freeze-status" class="%s font-semibold">%s</span>`, statusClass, statusText)
}
```

**Step 5: Register route in main.go**

Find the admin routes block and add:
```go
r.Post("/api/admin/settings/freeze", s.settingsH.SetScoreFreeze)
```

**Step 6: Add freeze UI to admin.html**

In the Settings tab of `internal/views/templates/admin.html`, add a "Competition" section before the existing settings form:

```html
<!-- Score Freeze -->
<div class="bg-gray-800 rounded-lg p-6 mb-6">
    <h3 class="text-lg font-semibold text-white mb-4">Score Freeze</h3>
    <form hx-post="/api/admin/settings/freeze" hx-target="#freeze-status" class="space-y-4">
        <div>
            <label class="text-xs text-gray-300 block mb-1">Freeze At (leave blank for immediate)</label>
            <input type="datetime-local" name="freeze_at"
                   class="bg-gray-700 border border-gray-600 rounded px-3 py-2 text-white text-sm focus:border-purple-500 focus:outline-none"
                   value="{{.FreezeAt}}">
        </div>
        <div class="flex items-center gap-3">
            <label class="text-xs text-gray-300">Freeze Enabled</label>
            <input type="hidden" name="freeze_enabled" value="0">
            <input type="checkbox" name="freeze_enabled" value="1" {{if .FreezeEnabled}}checked{{end}}
                   class="rounded">
        </div>
        <div class="flex items-center gap-3">
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">
                Save Freeze Settings
            </button>
            <span>Status: <span id="freeze-status" class="{{if .Frozen}}text-blue-400{{else}}text-green-400{{end}} font-semibold">
                {{if .Frozen}}Frozen{{else}}Live{{end}}
            </span></span>
        </div>
    </form>
</div>
```

Pass freeze data from the settings page handler in `main.go` (find `GET /admin` handler and add to template data):
```go
freezeEnabled, freezeAt, _ := db.GetScoreFreeze()
data["FreezeEnabled"] = freezeEnabled
data["Frozen"] = db.IsFrozen()
if freezeAt != nil {
    data["FreezeAt"] = freezeAt.Format("2006-01-02T15:04")
} else {
    data["FreezeAt"] = ""
}
```

**Step 7: Build and validate with agent-browser**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
npx agent-browser --session hctf2 open http://localhost:8092/login && \
npx agent-browser --session hctf2 fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2 fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2 find role button click --name Login && \
npx agent-browser --session hctf2 open http://localhost:8092/admin && \
npx agent-browser --session hctf2 screenshot --full /tmp/freeze-ui.png
```

Read `/tmp/freeze-ui.png` and verify the freeze section appears in Settings tab.

**Step 8: Commit**

```bash
git add internal/database/migrations/010_* internal/database/queries.go internal/handlers/settings.go internal/views/templates/admin.html main.go
git commit -m "feat(scoreboard): add score freeze with scheduled and manual modes

Admins can set a freeze datetime or toggle freeze manually.
Scoreboard queries filter submissions to before the freeze time.
Freeze status shown in admin dashboard Settings tab."
```

---

## Task 3: CTFtime.org JSON Export

**Files:**
- Modify: `internal/handlers/scoreboard.go` (add CTFtime handler)
- Modify: `main.go` (register route)
- Modify: `internal/views/templates/admin.html` (show endpoint URL in Settings)

**Step 1: Add CTFtime handler**

In `internal/handlers/scoreboard.go`, add:

```go
// CTFtimeExport handles GET /api/ctftime
// Returns scoreboard in CTFtime.org JSON format.
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
        name := ""
        if e.TeamName != nil {
            name = *e.TeamName
        } else {
            name = e.UserName
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
```

**Step 2: Register route in main.go**

In the public routes block (no auth required), add:
```go
r.Get("/api/ctftime", s.scoreboardH.CTFtimeExport)
```

**Step 3: Add CTFtime URL to admin Settings UI**

In `internal/views/templates/admin.html`, in the Settings tab, add after the freeze section:

```html
<!-- CTFtime Integration -->
<div class="bg-gray-800 rounded-lg p-6 mb-6">
    <h3 class="text-lg font-semibold text-white mb-2">CTFtime Integration</h3>
    <p class="text-sm text-gray-400 mb-3">Enter this URL in your CTFtime event scoreboard settings:</p>
    <div class="flex items-center gap-2">
        <code class="bg-gray-900 text-purple-300 px-3 py-2 rounded text-sm flex-1 select-all" id="ctftime-url">
            {{.BaseURL}}/api/ctftime
        </code>
        <button onclick="navigator.clipboard.writeText(document.getElementById('ctftime-url').textContent.trim())"
                class="px-3 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm transition">
            Copy
        </button>
    </div>
</div>
```

Pass `BaseURL` in admin page template data in `main.go`:
```go
// derive base URL from request
scheme := "http"
if r.TLS != nil {
    scheme = "https"
}
data["BaseURL"] = scheme + "://" + r.Host
```

**Step 4: Build and validate with agent-browser**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
# Test endpoint directly
curl -s http://localhost:8092/api/ctftime | python3 -m json.tool
# Validate UI
npx agent-browser --session hctf2 open http://localhost:8092/admin && \
npx agent-browser --session hctf2 screenshot --full /tmp/ctftime-ui.png
```

**Step 5: Commit**

```bash
git add internal/handlers/scoreboard.go internal/views/templates/admin.html main.go
git commit -m "feat(api): add CTFtime.org JSON scoreboard export at /api/ctftime

Public endpoint returning team standings in CTFtime format.
Respects score freeze if active. URL displayed in admin Settings tab."
```

---

## Task 4: File Attachments

### Task 4a: Storage Interface + Local Backend

**Files:**
- Create: `internal/storage/storage.go`
- Create: `internal/storage/local.go`
- Create: `internal/storage/storage_test.go`

**Step 1: Write failing test**

Create `internal/storage/storage_test.go`:
```go
package storage_test

import (
    "bytes"
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/yourusername/hctf2/internal/storage"
)

func TestLocalStorage_UploadAndDelete(t *testing.T) {
    dir := t.TempDir()
    s := storage.NewLocal(dir, "/uploads")

    content := []byte("hello world")
    url, err := s.Upload(context.Background(), "test.txt", bytes.NewReader(content))
    if err != nil {
        t.Fatalf("Upload failed: %v", err)
    }
    if !strings.HasPrefix(url, "/uploads/") {
        t.Fatalf("expected URL to start with /uploads/, got %s", url)
    }

    // Verify file exists
    filename := filepath.Base(url)
    if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
        t.Fatalf("file not found on disk: %v", err)
    }

    // Delete
    if err := s.Delete(context.Background(), url); err != nil {
        t.Fatalf("Delete failed: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, filename)); !os.IsNotExist(err) {
        t.Fatal("expected file to be deleted")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/... -v
```
Expected: FAIL with package not found.

**Step 3: Create storage interface**

Create `internal/storage/storage.go`:
```go
package storage

import (
    "context"
    "io"
)

// Storage handles file upload and deletion.
type Storage interface {
    Upload(ctx context.Context, filename string, r io.Reader) (url string, err error)
    Delete(ctx context.Context, url string) error
}
```

**Step 4: Create local backend**

Create `internal/storage/local.go`:
```go
package storage

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// LocalStorage stores files on disk and serves them via a URL prefix.
type LocalStorage struct {
    dir    string // absolute path to upload directory
    prefix string // URL prefix, e.g. "/uploads"
}

// NewLocal creates a LocalStorage writing to dir, serving at prefix.
func NewLocal(dir, prefix string) *LocalStorage {
    os.MkdirAll(dir, 0755)
    return &LocalStorage{dir: dir, prefix: prefix}
}

// Upload saves r to disk with a unique name derived from filename.
func (s *LocalStorage) Upload(ctx context.Context, filename string, r io.Reader) (string, error) {
    ext := filepath.Ext(filename)
    unique := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitize(strings.TrimSuffix(filename, ext)), ext)
    dest := filepath.Join(s.dir, unique)

    f, err := os.Create(dest)
    if err != nil {
        return "", fmt.Errorf("create file: %w", err)
    }
    defer f.Close()

    if _, err := io.Copy(f, r); err != nil {
        os.Remove(dest)
        return "", fmt.Errorf("write file: %w", err)
    }
    return s.prefix + "/" + unique, nil
}

// Delete removes the file corresponding to url from disk.
func (s *LocalStorage) Delete(ctx context.Context, url string) error {
    if !strings.HasPrefix(url, s.prefix+"/") {
        return nil // not a local file, ignore
    }
    filename := strings.TrimPrefix(url, s.prefix+"/")
    filename = filepath.Base(filename) // prevent path traversal
    return os.Remove(filepath.Join(s.dir, filename))
}

// sanitize removes unsafe characters from filenames.
func sanitize(name string) string {
    var b strings.Builder
    for _, r := range name {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
            b.WriteRune(r)
        } else {
            b.WriteRune('_')
        }
    }
    return b.String()
}
```

**Step 5: Run tests**

```bash
go test ./internal/storage/... -v
```
Expected: PASS.

### Task 4b: Wire Storage into Server + Upload Route

**Files:**
- Modify: `main.go` (add `--upload-dir` flag, wire storage, add upload route, serve /uploads/)
- Modify: `internal/handlers/challenges.go` (add Upload and DeleteFile handlers)

**Step 1: Add CLI flag and wire in main.go**

Add flag near other flags:
```go
uploadDir = flag.String("upload-dir", "./uploads", "Directory for file uploads")
```

Add to server struct:
```go
storage storage.Storage
```

Initialize after handler setup:
```go
s.storage = storage.NewLocal(*uploadDir, "/uploads")
```

Serve local uploads (authenticated):
```go
r.With(auth.RequireAuth).Get("/uploads/*", func(w http.ResponseWriter, r *http.Request) {
    // strip /uploads prefix and serve from uploadDir
    filename := chi.URLParam(r, "*")
    filename = filepath.Base(filename) // prevent traversal
    http.ServeFile(w, r, filepath.Join(*uploadDir, filename))
})
```

Import: `"github.com/yourusername/hctf2/internal/storage"`

**Step 2: Add upload handler to ChallengeHandler**

Update `ChallengeHandler` struct:
```go
type ChallengeHandler struct {
    db            *database.DB
    submitLimiter *ratelimit.Limiter
    storage       storage.Storage
}
```

Update constructor:
```go
func NewChallengeHandler(db *database.DB, limiter *ratelimit.Limiter, stor storage.Storage) *ChallengeHandler {
    return &ChallengeHandler{db: db, submitLimiter: limiter, storage: stor}
}
```

Add upload handler:
```go
// UploadChallengeFile handles POST /api/admin/challenges/{id}/upload
func (h *ChallengeHandler) UploadChallengeFile(w http.ResponseWriter, r *http.Request) {
    challengeID := chi.URLParam(r, "id")

    if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
        http.Error(w, "File too large (max 50MB)", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "No file provided", http.StatusBadRequest)
        return
    }
    defer file.Close()

    url, err := h.storage.Upload(r.Context(), header.Filename, file)
    if err != nil {
        http.Error(w, "Upload failed", http.StatusInternalServerError)
        return
    }

    if err := h.db.SetChallengeFileURL(challengeID, url); err != nil {
        h.storage.Delete(r.Context(), url)
        http.Error(w, "Failed to save", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/html")
    fmt.Fprintf(w, `<div class="text-green-400 text-sm">File uploaded: <a href="%s" class="underline" target="_blank">%s</a>
        <button hx-delete="/api/admin/challenges/%s/file" hx-target="#file-section-%s" class="ml-2 text-red-400 hover:text-red-300 text-xs">Remove</button>
    </div>`, url, header.Filename, challengeID, challengeID)
}

// DeleteChallengeFile handles DELETE /api/admin/challenges/{id}/file
func (h *ChallengeHandler) DeleteChallengeFile(w http.ResponseWriter, r *http.Request) {
    challengeID := chi.URLParam(r, "id")

    challenge, err := h.db.GetChallengeByID(challengeID)
    if err != nil {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }

    if challenge.FileURL != nil && *challenge.FileURL != "" {
        h.storage.Delete(r.Context(), *challenge.FileURL)
    }

    if err := h.db.SetChallengeFileURL(challengeID, ""); err != nil {
        http.Error(w, "Failed to update", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(`<div class="text-gray-400 text-sm">No file attached.</div>`))
}
```

**Step 3: Add SetChallengeFileURL to queries.go**

```go
// SetChallengeFileURL updates the file_url for a challenge.
func (db *DB) SetChallengeFileURL(challengeID, url string) error {
    var fileURL interface{}
    if url != "" {
        fileURL = url
    }
    _, err := db.Exec(`UPDATE challenges SET file_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, fileURL, challengeID)
    return err
}
```

**Step 4: Check if file_url column exists in challenges table**

```bash
grep -r "file_url" internal/database/migrations/
```

If not present, create migration 010 (shift freeze to 011):
```sql
-- internal/database/migrations/010_challenge_file_url.up.sql
ALTER TABLE challenges ADD COLUMN file_url TEXT;
```
```sql
-- internal/database/migrations/010_challenge_file_url.down.sql
-- SQLite doesn't support DROP COLUMN in older versions, no-op
SELECT 1;
```

**Step 5: Register routes in main.go**

```go
r.Post("/api/admin/challenges/{id}/upload", s.challengeH.UploadChallengeFile)
r.Delete("/api/admin/challenges/{id}/file", s.challengeH.DeleteChallengeFile)
```

**Step 6: Add file upload UI to admin.html**

In the challenge edit form in `admin.html`, add a file section:
```html
<div id="file-section-{{.ID}}" class="mt-3">
    {{if .FileURL}}
    <div class="text-green-400 text-sm">File: <a href="{{.FileURL}}" class="underline" target="_blank">{{.FileURL}}</a>
        <button hx-delete="/api/admin/challenges/{{.ID}}/file" hx-target="#file-section-{{.ID}}"
                class="ml-2 text-red-400 hover:text-red-300 text-xs">Remove</button>
    </div>
    {{else}}
    <label class="text-xs text-gray-300 block mb-1">Attach File (max 50MB)</label>
    <input type="file" name="file"
           hx-post="/api/admin/challenges/{{.ID}}/upload"
           hx-target="#file-section-{{.ID}}"
           hx-encoding="multipart/form-data"
           hx-trigger="change"
           class="text-sm text-gray-300">
    {{end}}
</div>
```

**Step 7: Build and validate with agent-browser**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
npx agent-browser --session hctf2 open http://localhost:8092/login && \
npx agent-browser --session hctf2 fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2 fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2 find role button click --name Login && \
npx agent-browser --session hctf2 open http://localhost:8092/admin && \
npx agent-browser --session hctf2 screenshot --full /tmp/file-upload-ui.png
```

Read the screenshot and verify the file upload input appears in the challenge edit form.

**Step 8: Commit**

```bash
git add internal/storage/ internal/handlers/challenges.go internal/database/queries.go internal/database/migrations/010_* internal/views/templates/admin.html main.go go.mod go.sum
git commit -m "feat(challenges): add file attachment support with local storage

Challenges can have file attachments uploaded via admin dashboard.
Files stored in --upload-dir (default ./uploads), served authenticated at /uploads/*.
S3 backend architecture ready but not yet wired (future task)."
```

---

## Task 5: Challenge Import/Export

**Files:**
- Create: `internal/handlers/importexport.go`
- Modify: `internal/database/queries.go` (add bulk export/import queries)
- Modify: `main.go` (register routes)
- Modify: `internal/views/templates/admin.html` (export button + import form)

**Step 1: Define import/export structs**

In `internal/handlers/importexport.go`:

```go
package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/yourusername/hctf2/internal/database"
)

type ImportExportHandler struct {
    db *database.DB
}

func NewImportExportHandler(db *database.DB) *ImportExportHandler {
    return &ImportExportHandler{db: db}
}

// Export format structs
type exportHint struct {
    Content string `json:"content"`
    Cost    int    `json:"cost"`
    Order   int    `json:"order"`
}

type exportQuestion struct {
    Name          string       `json:"name"`
    Description   string       `json:"description"`
    Flag          string       `json:"flag"`
    FlagMask      string       `json:"flag_mask,omitempty"`
    CaseSensitive bool         `json:"case_sensitive"`
    Points        int          `json:"points"`
    FileURL       string       `json:"file_url,omitempty"`
    Hints         []exportHint `json:"hints,omitempty"`
}

type exportChallenge struct {
    Name           string           `json:"name"`
    Description    string           `json:"description"`
    Category       string           `json:"category"`
    Difficulty     string           `json:"difficulty"`
    Visible        bool             `json:"visible"`
    DynamicScoring bool             `json:"dynamic_scoring"`
    InitialPoints  int              `json:"initial_points"`
    MinimumPoints  int              `json:"minimum_points"`
    DecayThreshold int              `json:"decay_threshold"`
    Questions      []exportQuestion `json:"questions"`
}

type exportBundle struct {
    Version      int               `json:"version"`
    ExportedAt   time.Time         `json:"exported_at"`
    Categories   []string          `json:"categories"`
    Difficulties []string          `json:"difficulties"`
    Challenges   []exportChallenge `json:"challenges"`
}

type importResult struct {
    Imported int      `json:"imported"`
    Renamed  []string `json:"renamed"`
    Errors   []string `json:"errors"`
}
```

**Step 2: Add Export handler**

```go
// ExportChallenges handles GET /api/admin/export
func (h *ImportExportHandler) ExportChallenges(w http.ResponseWriter, r *http.Request) {
    bundle, err := h.db.ExportBundle()
    if err != nil {
        http.Error(w, "Export failed", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hctf2-export-%s.json"`, time.Now().Format("2006-01-02")))
    json.NewEncoder(w).Encode(bundle)
}
```

**Step 3: Add Import handler**

```go
// ImportChallenges handles POST /api/admin/import
func (h *ImportExportHandler) ImportChallenges(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    file, _, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "No file provided", http.StatusBadRequest)
        return
    }
    defer file.Close()

    var bundle exportBundle
    if err := json.NewDecoder(file).Decode(&bundle); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    if bundle.Version != 1 {
        http.Error(w, "Unsupported export version", http.StatusBadRequest)
        return
    }

    result, err := h.db.ImportBundle(bundle.Categories, bundle.Difficulties, bundle.Challenges)
    if err != nil {
        http.Error(w, "Import failed: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Return HTMX-friendly result
    w.Header().Set("Content-Type", "text/html")
    html := fmt.Sprintf(`<div class="p-4 bg-green-900/50 border border-green-700 rounded text-sm text-green-300">
        Imported %d challenge(s).`, result.Imported)
    if len(result.Renamed) > 0 {
        html += `</div><div class="mt-2 p-4 bg-yellow-900/50 border border-yellow-700 rounded text-sm text-yellow-300"><strong>Renamed (duplicates):</strong><ul class="list-disc ml-4 mt-1">`
        for _, r := range result.Renamed {
            html += fmt.Sprintf("<li>%s</li>", r)
        }
        html += `</ul>`
    }
    if len(result.Errors) > 0 {
        html += `</div><div class="mt-2 p-4 bg-red-900/50 border border-red-700 rounded text-sm text-red-300"><strong>Errors:</strong><ul class="list-disc ml-4 mt-1">`
        for _, e := range result.Errors {
            html += fmt.Sprintf("<li>%s</li>", e)
        }
        html += `</ul>`
    }
    html += `</div>`
    w.Write([]byte(html))
}
```

**Step 4: Add ExportBundle and ImportBundle to queries.go**

```go
// ExportBundle builds the full export payload.
func (db *DB) ExportBundle() (*exportBundle, error) {
    // NOTE: exportBundle type is in handlers package; use a shared type or re-define inline.
    // For simplicity, define inline structs here or move export types to models package.
    // Recommended: move export types to internal/models/export.go
    ...
}
```

**Important**: Move export structs (`exportBundle`, `exportChallenge`, etc.) to `internal/models/export.go` so both `handlers` and `database` packages can use them without circular imports.

Create `internal/models/export.go`:
```go
package models

import "time"

type ExportHint struct {
    Content string `json:"content"`
    Cost    int    `json:"cost"`
    Order   int    `json:"order"`
}

type ExportQuestion struct {
    Name          string       `json:"name"`
    Description   string       `json:"description"`
    Flag          string       `json:"flag"`
    FlagMask      string       `json:"flag_mask,omitempty"`
    CaseSensitive bool         `json:"case_sensitive"`
    Points        int          `json:"points"`
    FileURL       string       `json:"file_url,omitempty"`
    Hints         []ExportHint `json:"hints,omitempty"`
}

type ExportChallenge struct {
    Name           string           `json:"name"`
    Description    string           `json:"description"`
    Category       string           `json:"category"`
    Difficulty     string           `json:"difficulty"`
    Visible        bool             `json:"visible"`
    DynamicScoring bool             `json:"dynamic_scoring"`
    InitialPoints  int              `json:"initial_points"`
    MinimumPoints  int              `json:"minimum_points"`
    DecayThreshold int              `json:"decay_threshold"`
    Questions      []ExportQuestion `json:"questions"`
}

type ExportBundle struct {
    Version      int               `json:"version"`
    ExportedAt   time.Time         `json:"exported_at"`
    Categories   []string          `json:"categories"`
    Difficulties []string          `json:"difficulties"`
    Challenges   []ExportChallenge `json:"challenges"`
}

type ImportResult struct {
    Imported int      `json:"imported"`
    Renamed  []string `json:"renamed"`
    Errors   []string `json:"errors"`
}
```

Then implement `ExportBundle()` in `queries.go`:
```go
func (db *DB) ExportBundle() (*models.ExportBundle, error) {
    bundle := &models.ExportBundle{
        Version:    1,
        ExportedAt: time.Now(),
    }

    // Categories
    cats, _ := db.GetCategories()
    for _, c := range cats {
        bundle.Categories = append(bundle.Categories, c.Name)
    }

    // Difficulties
    diffs, _ := db.GetDifficulties()
    for _, d := range diffs {
        bundle.Difficulties = append(bundle.Difficulties, d.Name)
    }

    // Challenges
    challenges, err := db.GetChallenges(false) // include hidden
    if err != nil {
        return nil, err
    }

    for _, c := range challenges {
        ec := models.ExportChallenge{
            Name:           c.Name,
            Description:    c.Description,
            Category:       c.Category,
            Difficulty:     c.Difficulty,
            Visible:        c.Visible,
            DynamicScoring: c.DynamicScoring,
            InitialPoints:  c.InitialPoints,
            MinimumPoints:  c.MinimumPoints,
            DecayThreshold: c.DecayThreshold,
        }

        questions, err := db.GetQuestions(c.ID)
        if err != nil {
            continue
        }
        for _, q := range questions {
            eq := models.ExportQuestion{
                Name:          q.Name,
                Description:   q.Description,
                Flag:          q.Flag,
                CaseSensitive: q.CaseSensitive,
                Points:        q.Points,
            }
            if q.FlagMask != nil {
                eq.FlagMask = *q.FlagMask
            }
            if q.FileURL != nil {
                eq.FileURL = *q.FileURL
            }

            hints, _ := db.GetHintsForQuestion(q.ID)
            for _, h := range hints {
                eq.Hints = append(eq.Hints, models.ExportHint{
                    Content: h.Content,
                    Cost:    h.Cost,
                    Order:   h.Order,
                })
            }
            ec.Questions = append(ec.Questions, eq)
        }
        bundle.Challenges = append(bundle.Challenges, ec)
    }

    return bundle, nil
}
```

Implement `ImportBundle()` in `queries.go`:
```go
func (db *DB) ImportBundle(categories, difficulties []string, challenges []models.ExportChallenge) (*models.ImportResult, error) {
    result := &models.ImportResult{}

    // Ensure categories exist
    for _, cat := range categories {
        db.Exec(`INSERT OR IGNORE INTO categories (id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, NewID(), cat)
    }
    for _, diff := range difficulties {
        db.Exec(`INSERT OR IGNORE INTO difficulties (id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, NewID(), diff)
    }

    for _, ec := range challenges {
        // Handle duplicate names
        name := ec.Name
        for i := 2; ; i++ {
            var count int
            db.QueryRow(`SELECT COUNT(*) FROM challenges WHERE name = ?`, name).Scan(&count)
            if count == 0 {
                break
            }
            if i == 2 && name == ec.Name {
                result.Renamed = append(result.Renamed, fmt.Sprintf("%s → %s (%d)", ec.Name, ec.Name, i))
            }
            name = fmt.Sprintf("%s (%d)", ec.Name, i)
        }

        cID := NewID()
        _, err := db.Exec(`
            INSERT INTO challenges (id, name, description, category, difficulty, visible,
                dynamic_scoring, initial_points, minimum_points, decay_threshold, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
            cID, name, ec.Description, ec.Category, ec.Difficulty, ec.Visible,
            ec.DynamicScoring, ec.InitialPoints, ec.MinimumPoints, ec.DecayThreshold)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("Failed to import %q: %v", ec.Name, err))
            continue
        }

        for _, eq := range ec.Questions {
            qID := NewID()
            var fileURL interface{}
            if eq.FileURL != "" {
                fileURL = eq.FileURL
            }
            db.Exec(`
                INSERT INTO questions (id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
                qID, cID, eq.Name, eq.Description, eq.Flag, eq.FlagMask, eq.CaseSensitive, eq.Points, fileURL)

            for _, eh := range eq.Hints {
                db.Exec(`
                    INSERT INTO hints (id, question_id, content, cost, "order", created_at)
                    VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
                    NewID(), qID, eh.Content, eh.Cost, eh.Order)
            }
        }
        result.Imported++
    }

    return result, nil
}
```

**Step 5: Register routes in main.go**

```go
s.importExportH = handlers.NewImportExportHandler(db)
// ...
r.Get("/api/admin/export", s.importExportH.ExportChallenges)
r.Post("/api/admin/import", s.importExportH.ImportChallenges)
```

**Step 6: Add import/export UI to admin.html**

In the Settings tab:
```html
<!-- Import / Export -->
<div class="bg-gray-800 rounded-lg p-6 mb-6">
    <h3 class="text-lg font-semibold text-white mb-4">Challenge Import / Export</h3>
    <div class="flex flex-col gap-4">
        <!-- Export -->
        <div>
            <a href="/api/admin/export" download
               class="inline-block px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded text-sm font-medium transition">
                Export All Challenges (JSON)
            </a>
            <p class="text-xs text-gray-400 mt-1">Downloads a JSON file with all challenges, questions, hints, categories and difficulties.</p>
        </div>
        <!-- Import -->
        <div>
            <label class="text-xs text-gray-300 block mb-1">Import Challenges (JSON)</label>
            <input type="file" name="file" accept=".json"
                   hx-post="/api/admin/import"
                   hx-target="#import-result"
                   hx-encoding="multipart/form-data"
                   hx-trigger="change"
                   class="text-sm text-gray-300 block mb-2">
            <div id="import-result"></div>
        </div>
    </div>
</div>
```

**Step 7: Build and validate with agent-browser**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
# Test export
curl -s http://localhost:8092/api/admin/export \
  -H "Cookie: $(npx agent-browser --session hctf2 cookies | grep token)" | python3 -m json.tool
# Validate UI
npx agent-browser --session hctf2 open http://localhost:8092/admin && \
npx agent-browser --session hctf2 screenshot --full /tmp/import-export-ui.png
```

**Step 8: Commit**

```bash
git add internal/handlers/importexport.go internal/models/export.go internal/database/queries.go internal/views/templates/admin.html main.go
git commit -m "feat(admin): add challenge import/export in JSON format

Export: GET /api/admin/export downloads full bundle (challenges, questions, hints, categories, difficulties).
Import: POST /api/admin/import creates all entities, auto-renames duplicates with (2), (3) suffix.
Renamed items shown in yellow warning box after import."
```

---

## Task 6: Accessibility — Skip-to-Main Link

**Files:**
- Modify: `internal/views/templates/base.html`

**Step 1: Add skip link as first element in body**

Open `internal/views/templates/base.html`. Find the opening `<body` tag. Immediately after the `<body ...>` tag, before any other content, add:

```html
<a href="#main-content"
   class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-purple-600 focus:text-white focus:rounded focus:outline-none">
  Skip to main content
</a>
```

**Step 2: Add id to main element**

Find the `<main` tag in `base.html` and add `id="main-content"`:
```html
<main id="main-content" ...existing classes...>
```

**Step 3: Build and validate**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
npx agent-browser --session hctf2 open http://localhost:8092 && \
npx agent-browser --session hctf2 screenshot --full /tmp/skip-link.png
```

Read the screenshot — the skip link should be invisible normally. To verify it appears on focus, check DOM with:
```bash
npx agent-browser --session hctf2 snapshot
```
Verify `<a href="#main-content">` is the first element.

**Step 4: Commit**

```bash
git add internal/views/templates/base.html
git commit -m "feat(a11y): add skip-to-main-content link for keyboard navigation

Hidden by default, visible on Tab focus. Meets WCAG 2.4.1."
```

---

## Task 7: Accessibility — Focus Trap in Modals

**Files:**
- Create: `internal/views/static/js/focus-trap.js`
- Modify: `internal/views/templates/base.html` (include script)
- Modify: `internal/views/templates/teams.html` (apply to team modal)
- Modify: `internal/views/templates/admin.html` (apply to edit modals)

**Step 1: Create focus trap utility**

Create `internal/views/static/js/focus-trap.js`:
```js
/**
 * Focus trap for modal dialogs.
 * Usage: <div x-data="focusTrap()" x-init="init($el)" @keydown.tab.prevent="handleTab($event)">
 *
 * Or simpler: call trapFocus(modalEl) on open, releaseFocus() on close.
 */

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function trapFocus(modal) {
    const focusable = [...modal.querySelectorAll(FOCUSABLE)];
    if (!focusable.length) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    modal._trapHandler = (e) => {
        if (e.key !== 'Tab') return;
        if (e.shiftKey) {
            if (document.activeElement === first) {
                e.preventDefault();
                last.focus();
            }
        } else {
            if (document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        }
    };

    modal.addEventListener('keydown', modal._trapHandler);
    first.focus();
}

function releaseFocus(modal, returnTo) {
    if (modal._trapHandler) {
        modal.removeEventListener('keydown', modal._trapHandler);
        delete modal._trapHandler;
    }
    if (returnTo) returnTo.focus();
}
```

**Step 2: Include script in base.html**

In `base.html`, before closing `</body>`, add:
```html
<script src="/static/js/focus-trap.js"></script>
```

**Step 3: Apply to teams modal**

In `internal/views/templates/teams.html`, find the modal div with `x-show`. Add `x-init` and watch for show/hide:

```html
<div x-show="$store.modal.open"
     x-effect="$store.modal.open ? trapFocus($el) : releaseFocus($el, $store.modal.trigger)"
     role="dialog"
     aria-modal="true"
     ...existing classes...>
```

Store the trigger element when opening:
```js
// When opening a modal button, store the trigger:
// $store.modal.trigger = $el before setting $store.modal.open = true
```

Find all modal-open buttons in teams.html and update:
```html
<button @click="$store.modal.trigger = $el; $store.modal.open = true" ...>
```

**Step 4: Apply to admin edit forms**

Admin edit forms use `x-show` inline within the list items, not full-screen modals — focus trap is less critical there. Add `autofocus` to the first input in each edit form as a minimum:

In `admin.html`, for challenge edit form:
```html
<input autofocus name="name" ...>
```
For question edit form:
```html
<input autofocus name="name" ...>
```

**Step 5: Build and validate with agent-browser**

```bash
task rebuild
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db --admin-email admin@test.com --admin-password testpass123 &
npx agent-browser --session hctf2 open http://localhost:8092/teams && \
npx agent-browser --session hctf2 screenshot --full /tmp/focus-trap.png
```

Read the screenshot and verify modal looks correct. Also check browser console:
```bash
npx agent-browser --session hctf2 console
```

**Step 6: Commit**

```bash
git add internal/views/static/js/focus-trap.js internal/views/templates/base.html internal/views/templates/teams.html internal/views/templates/admin.html
git commit -m "feat(a11y): add focus trap for modal dialogs

Tab/Shift+Tab cycles within open modal only.
Focus returns to trigger element on close.
autofocus added to first field in edit forms."
```

---

## Final: Run Full Test Suite

```bash
go test ./... -v
task rebuild
# Smoke test all new endpoints
curl -s http://localhost:8092/api/ctftime
curl -s http://localhost:8092/health
```

Then commit any final fixes.
