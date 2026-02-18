# hCTF2 New Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 5 new features: User Management (admin), Challenge Completion Styling, SQL Playground for Challenges, Public User Profiles, and Dark/Light Theme Toggle.

**Architecture:** Minimal changes following existing patterns - add handlers to `internal/handlers/`, routes to `main.go`, templates to `internal/views/templates/`, and database queries to `internal/database/queries.go`. Use HTMX + Alpine.js for interactivity, Tailwind for styling.

**Tech Stack:** Go 1.24+, Chi router, SQLite (modernc.org/sqlite), HTMX, Alpine.js, Tailwind CSS, DuckDB WASM

---

## Feature Overview & Version Plan

| Feature | Version Bump | Commit Type | Scope |
|---------|--------------|-------------|-------|
| User Management Admin Panel | minor (0.3.0) | feat | admin, users |
| Challenge Completion Styling | minor (0.3.0) | feat | challenges |
| SQL Playground for Challenges | minor (0.4.0) | feat | challenges, sql |
| Public User Profiles | minor (0.4.0) | feat | profile |
| Dark/Light Theme Toggle | minor (0.5.0) | feat | ui, theme |

---

## Task 1: User Management Admin Panel

**Description:** Add a "Users" tab in admin dashboard to view all users, toggle admin status, and manage users.

**Files:**
- Create: N/A (modifications only)
- Modify: `internal/database/queries.go` (add user queries)
- Modify: `internal/handlers/settings.go` (add user handler methods)
- Modify: `main.go` (add routes)
- Modify: `internal/views/templates/admin.html` (add users tab)

**Step 1: Add database queries for user management**

File: `internal/database/queries.go`

Add these methods to the DB struct:

```go
// GetAllUsers returns all users with basic info
func (db *DB) GetAllUsers() ([]models.User, error) {
    query := `SELECT id, email, name, is_admin, created_at, updated_at FROM users ORDER BY created_at DESC`
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []models.User
    for rows.Next() {
        var u models.User
        err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
        if err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, nil
}

// SetUserAdminStatus updates a user's admin status
func (db *DB) SetUserAdminStatus(userID string, isAdmin bool) error {
    query := `UPDATE users SET is_admin = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
    _, err := db.Exec(query, isAdmin, userID)
    return err
}

// DeleteUser deletes a user and their related data
func (db *DB) DeleteUser(userID string) error {
    // Due to CASCADE constraints, this will also delete:
    // - submissions, hint_unlocks
    // Teams will have owner_id set to NULL (ON DELETE SET NULL)
    query := `DELETE FROM users WHERE id = ?`
    _, err := db.Exec(query, userID)
    return err
}
```

**Step 2: Add user handler methods to SettingsHandler**

File: `internal/handlers/settings.go`

Add these methods:

```go
// ListUsers returns all users (admin only)
func (h *SettingsHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    users, err := h.db.GetAllUsers()
    if err != nil {
        http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

// UpdateUserAdmin toggles admin status for a user
func (h *SettingsHandler) UpdateUserAdmin(w http.ResponseWriter, r *http.Request) {
    userID := chi.URLParam(r, "id")
    
    // Prevent self-demotion check happens in handler
    currentUser := auth.GetUserFromContext(r.Context())
    if currentUser != nil && currentUser.UserID == userID {
        http.Error(w, "Cannot modify your own admin status", http.StatusBadRequest)
        return
    }
    
    isAdmin := r.FormValue("is_admin") == "true"
    
    if err := h.db.SetUserAdminStatus(userID, isAdmin); err != nil {
        http.Error(w, "Failed to update user", http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}

// DeleteUser deletes a user
func (h *SettingsHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
    userID := chi.URLParam(r, "id")
    
    // Prevent self-deletion
    currentUser := auth.GetUserFromContext(r.Context())
    if currentUser != nil && currentUser.UserID == userID {
        http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
        return
    }
    
    if err := h.db.DeleteUser(userID); err != nil {
        http.Error(w, "Failed to delete user", http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}
```

Add import for chi if not present: `"github.com/go-chi/chi/v5"`

**Step 3: Add API routes**

File: `main.go`

In the admin API routes section (around line 291), add:

```go
// API routes - Admin (protected)
r.Group(func(r chi.Router) {
    r.Use(s.requireAdmin)
    // ... existing routes ...
    
    // User management routes
    r.Get("/api/admin/users", s.settingsH.ListUsers)
    r.Put("/api/admin/users/{id}/admin", s.settingsH.UpdateUserAdmin)
    r.Delete("/api/admin/users/{id}", s.settingsH.DeleteUser)
})
```

**Step 4: Update admin handler to fetch users**

File: `main.go` - `handleAdminDashboard` function

Add to the data map:

```go
users, _ := s.db.GetAllUsers()
// ... existing data ...
data := map[string]interface{}{
    // ... existing fields ...
    "Users": users,
}
```

**Step 5: Add Users tab to admin template**

File: `internal/views/templates/admin.html`

1. Add the tab button after the Settings tab (around line 44):

```html
<button
    @click="tab = 'users'"
    :class="tab === 'users' ? 'border-b-2 border-purple-500 text-purple-400' : 'text-gray-400 hover:text-gray-300'"
    class="pb-3 font-medium transition">
    Users
</button>
```

2. Add the Users tab content at the end of the main div (before `{{end}}`):

```html
<!-- Users Tab -->
<div x-show="tab === 'users'" class="space-y-6">
    <h2 class="text-2xl font-bold text-white mb-4">User Management</h2>
    
    <div id="users-list" class="space-y-2">
        {{range .Users}}
        <div id="user-{{.ID}}" class="bg-dark-surface border border-dark-border rounded-lg px-4 py-3">
            <div class="flex items-center justify-between">
                <div>
                    <p class="text-white font-medium">{{.Name}}</p>
                    <p class="text-sm text-gray-400">{{.Email}} • Joined {{.CreatedAt.Format "Jan 2, 2006"}}</p>
                </div>
                <div class="flex items-center gap-4">
                    {{if .IsAdmin}}
                    <span class="px-2 py-1 bg-purple-600 text-white text-xs rounded">Admin</span>
                    {{else}}
                    <span class="px-2 py-1 bg-gray-600 text-gray-200 text-xs rounded">User</span>
                    {{end}}
                    <button 
                        hx-put="/api/admin/users/{{.ID}}/admin"
                        hx-vals='{"is_admin": {{if .IsAdmin}}false{{else}}true{{end}}}'
                        hx-target="#user-{{.ID}}"
                        hx-swap="outerHTML"
                        hx-confirm="{{if .IsAdmin}}Remove admin privileges from {{.Name}}?{{else}}Make {{.Name}} an admin?{{end}}"
                        class="px-3 py-1 {{if .IsAdmin}}bg-yellow-600 hover:bg-yellow-700{{else}}bg-green-600 hover:bg-green-700{{end}} text-white rounded text-xs">
                        {{if .IsAdmin}}Demote{{else}}Promote{{end}}
                    </button>
                    <button 
                        hx-delete="/api/admin/users/{{.ID}}"
                        hx-target="#user-{{.ID}}"
                        hx-swap="delete swap:0.3s"
                        hx-confirm="Delete user {{.Name}}? This cannot be undone."
                        class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs">
                        Delete
                    </button>
                </div>
            </div>
        </div>
        {{end}}
        {{if not .Users}}
        <div class="text-center py-8 text-gray-400">
            <p>No users found.</p>
        </div>
        {{end}}
    </div>
</div>
```

**Step 6: Commit**

```bash
git add internal/database/queries.go internal/handlers/settings.go main.go internal/views/templates/admin.html
git commit -m "feat(admin): add user management panel

- Add GetAllUsers, SetUserAdminStatus, DeleteUser queries
- Add ListUsers, UpdateUserAdmin, DeleteUser handlers
- Add users tab to admin dashboard with promote/demote/delete
- Prevent self-modification for safety"
```

---

## Task 2: Challenge Completion Styling

**Description:** When all questions in a challenge are solved, stylize the challenge card to show completion status.

**Files:**
- Modify: `internal/database/queries.go` (add completion check query)
- Modify: `main.go` (update handleChallenges to include completion data)
- Modify: `internal/views/templates/challenges.html` (add completion styling)

**Step 1: Add query to check challenge completion for a user**

File: `internal/database/queries.go`

Add this method:

```go
// ChallengeCompletion tracks completion status for a challenge
type ChallengeCompletion struct {
    ChallengeID    string
    TotalQuestions int
    SolvedQuestions int
    IsComplete     bool
}

// GetChallengeCompletionForUser returns completion status for all challenges for a user
func (db *DB) GetChallengeCompletionForUser(userID string) (map[string]*ChallengeCompletion, error) {
    query := `
        SELECT 
            c.id as challenge_id,
            COUNT(DISTINCT q.id) as total_questions,
            COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.question_id END) as solved_questions
        FROM challenges c
        LEFT JOIN questions q ON c.id = q.challenge_id
        LEFT JOIN submissions s ON q.id = s.question_id AND s.user_id = ? AND s.is_correct = 1
        WHERE c.visible = 1
        GROUP BY c.id
    `
    rows, err := db.Query(query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    completions := make(map[string]*ChallengeCompletion)
    for rows.Next() {
        var cc ChallengeCompletion
        err := rows.Scan(&cc.ChallengeID, &cc.TotalQuestions, &cc.SolvedQuestions)
        if err != nil {
            return nil, err
        }
        cc.IsComplete = cc.TotalQuestions > 0 && cc.SolvedQuestions == cc.TotalQuestions
        completions[cc.ChallengeID] = &cc
    }
    return completions, nil
}
```

**Step 2: Update challenges handler to include completion data**

File: `main.go` - `handleChallenges` function

Update the data map to include completion info:

```go
func (s *Server) handleChallenges(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetUserFromContext(r.Context())
    visibleOnly := claims == nil || !claims.IsAdmin

    challenges, err := s.db.GetChallenges(visibleOnly)
    if err != nil {
        http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
        return
    }

    categories, _ := s.db.GetAllCategories()
    difficulties, _ := s.db.GetAllDifficulties()
    customCode, _ := s.db.GetCustomCode("challenges")

    // Get completion data for logged-in users
    var completions map[string]*database.ChallengeCompletion
    if claims != nil {
        completions, _ = s.db.GetChallengeCompletionForUser(claims.UserID)
    }

    data := map[string]interface{}{
        "Title":        "Challenges",
        "Page":         "challenges",
        "User":         claims,
        "Challenges":   challenges,
        "Categories":   categories,
        "Difficulties": difficulties,
        "CustomCode":   customCode,
        "Completions":  completions,
    }
    s.render(w, "base.html", data)
}
```

**Step 3: Update challenges template with completion styling**

File: `internal/views/templates/challenges.html`

Update the challenge card to show completion status:

```html
{{range .Challenges}}
{{$completed := false}}
{{$total := 0}}
{{$solved := 0}}
{{if $.Completions}}
    {{with index $.Completions .ID}}
        {{$completed = .IsComplete}}
        {{$total = .TotalQuestions}}
        {{$solved = .SolvedQuestions}}
    {{end}}
{{end}}
<div class="{{if $completed}}bg-green-900/20 border-green-600{{else}}bg-dark-surface border-dark-border{{end}} border rounded-lg p-6 hover:border-blue-500 transition relative"
     data-search-name="{{.Name}}"
     data-search-desc="{{stripMarkdown .Description}}"
     data-categories="{{.Category}}"
     x-show="(category === 'all' || $el.dataset.categories.split(',').map(s=>s.trim()).includes(category)) && (difficulty === 'all' || difficulty === '{{.Difficulty}}') && (searchQuery === '' || $el.dataset.searchName.toLowerCase().includes(searchQuery.toLowerCase()) || $el.dataset.searchDesc.toLowerCase().includes(searchQuery.toLowerCase()))">
    {{if $completed}}
    <div class="absolute top-2 right-2">
        <span class="text-green-400" title="Completed!">✓</span>
    </div>
    {{end}}
    <div class="flex justify-between items-start mb-4">
        <h3 class="text-xl font-bold {{if $completed}}text-green-400{{else}}text-white{{end}}">{{.Name}}</h3>
        <span class="px-2 py-1 text-xs rounded {{difficultyBadge .Difficulty}}">
            {{.Difficulty}}
        </span>
    </div>
    <p class="text-sm text-gray-400 mb-2">
        {{range $i, $cat := splitCategories .Category}}{{if $i}}, {{end}}<span class="text-blue-400">{{$cat}}</span>{{end}}
    </p>
    <div class="text-gray-300 mb-4 line-clamp-3 prose prose-invert prose-sm max-w-none">{{markdown .Description}}</div>
    {{if $.User}}
    <div class="mb-3">
        <div class="w-full bg-dark-bg rounded-full h-2">
            <div class="{{if $completed}}bg-green-500{{else}}bg-blue-500{{end}} h-2 rounded-full transition-all" style="width: {{if gt $total 0}}{{div (mul $solved 100) $total}}{{else}}0{{end}}%"></div>
        </div>
        <p class="text-xs text-gray-400 mt-1">{{$solved}}/{{$total}} questions solved</p>
    </div>
    {{end}}
    <a href="/challenges/{{.ID}}" class="inline-block px-4 py-2 {{if $completed}}bg-green-600 hover:bg-green-700{{else}}bg-blue-600 hover:bg-blue-700{{end}} text-white rounded text-sm">
        {{if $completed}}View Challenge{{else}}Solve Challenge{{end}}
    </a>
</div>
{{end}}
```

**Step 4: Commit**

```bash
git add internal/database/queries.go main.go internal/views/templates/challenges.html
git commit -m "feat(challenges): add completion progress indicators

- Add ChallengeCompletion struct and GetChallengeCompletionForUser query
- Show progress bar and completion checkmark on challenge cards
- Style completed challenges with green border and text"
```

---

## Task 3: SQL Playground for Challenges

**Description:** Allow challenges to optionally use the SQL Playground engine for solving. Admin can enable SQL mode per challenge with a dataset URL.

**Files:**
- Modify: `internal/database/migrations/007_challenge_sql.down.sql` (create)
- Modify: `internal/database/migrations/007_challenge_sql.up.sql` (create)
- Modify: `internal/models/models.go` (add SQL fields to Challenge)
- Modify: `internal/database/queries.go` (update challenge queries)
- Modify: `internal/views/templates/admin.html` (add SQL options to challenge forms)
- Modify: `internal/views/templates/challenge.html` (add SQL playground when enabled)
- Modify: `internal/handlers/challenges.go` (handle SQL challenge submissions)

**Step 1: Create database migration**

File: `internal/database/migrations/007_challenge_sql.up.sql`

```sql
-- Add SQL playground support to challenges
ALTER TABLE challenges ADD COLUMN sql_enabled BOOLEAN DEFAULT 0;
ALTER TABLE challenges ADD COLUMN sql_dataset_url TEXT;
ALTER TABLE challenges ADD COLUMN sql_schema_hint TEXT;

-- Add expected_query_result for SQL-based flag validation
ALTER TABLE questions ADD COLUMN expected_query_result TEXT;
```

File: `internal/database/migrations/007_challenge_sql.down.sql`

```sql
-- Remove SQL playground columns
ALTER TABLE challenges DROP COLUMN sql_enabled;
ALTER TABLE challenges DROP COLUMN sql_dataset_url;
ALTER TABLE challenges DROP COLUMN sql_schema_hint;
ALTER TABLE questions DROP COLUMN expected_query_result;
```

**Step 2: Update Challenge model**

File: `internal/models/models.go`

Update the Challenge struct:

```go
type Challenge struct {
    ID            string    `json:"id"`
    Name          string    `json:"name"`
    Description   string    `json:"description"`
    Category      string    `json:"category"`
    Difficulty    string    `json:"difficulty"`
    Tags          *string   `json:"tags,omitempty"`
    Visible       bool      `json:"visible"`
    SQLEnabled    bool      `json:"sql_enabled"`      // NEW
    SQLDatasetURL *string   `json:"sql_dataset_url,omitempty"` // NEW
    SQLSchemaHint *string   `json:"sql_schema_hint,omitempty"` // NEW
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

Update the Question struct:

```go
type Question struct {
    ID                 string    `json:"id"`
    ChallengeID        string    `json:"challenge_id"`
    Name               string    `json:"name"`
    Description        string    `json:"description"`
    Flag               string    `json:"-"`
    FlagMask           *string   `json:"flag_mask,omitempty"`
    CaseSensitive      bool      `json:"case_sensitive"`
    Points             int       `json:"points"`
    FileURL            *string   `json:"file_url,omitempty"`
    ExpectedQueryResult *string  `json:"expected_query_result,omitempty"` // NEW - for SQL challenges
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}
```

**Step 3: Update database queries**

File: `internal/database/queries.go`

Update GetChallengeByID query to include new columns:

```go
func (db *DB) GetChallengeByID(id string) (*models.Challenge, error) {
    query := `SELECT id, name, description, category, difficulty, tags, visible, 
              sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at 
              FROM challenges WHERE id = ?`
    var c models.Challenge
    var sqlEnabled int
    err := db.QueryRow(query, id).Scan(
        &c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty,
        &c.Tags, &c.Visible, &sqlEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint,
        &c.CreatedAt, &c.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    c.SQLEnabled = sqlEnabled == 1
    return &c, nil
}
```

Update GetChallenges query:

```go
func (db *DB) GetChallenges(visibleOnly bool) ([]models.Challenge, error) {
    query := `SELECT id, name, description, category, difficulty, tags, visible,
              sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at 
              FROM challenges`
    if visibleOnly {
        query += ` WHERE visible = 1`
    }
    query += ` ORDER BY created_at DESC`
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var challenges []models.Challenge
    for rows.Next() {
        var c models.Challenge
        var sqlEnabled int
        err := rows.Scan(
            &c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty,
            &c.Tags, &c.Visible, &sqlEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint,
            &c.CreatedAt, &c.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        c.SQLEnabled = sqlEnabled == 1
        challenges = append(challenges, c)
    }
    return challenges, nil
}
```

Update CreateChallenge:

```go
func (db *DB) CreateChallenge(name, description, category, difficulty string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string) (*models.Challenge, error) {
    query := `INSERT INTO challenges (name, description, category, difficulty, visible, sql_enabled, sql_dataset_url, sql_schema_hint) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?) 
              RETURNING id, created_at, updated_at`
    var c models.Challenge
    var sqlEnabledInt int
    if sqlEnabled {
        sqlEnabledInt = 1
    }
    err := db.QueryRow(query, name, description, category, difficulty, visible, sqlEnabledInt, sqlDatasetURL, sqlSchemaHint).Scan(
        &c.ID, &c.CreatedAt, &c.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    c.Name = name
    c.Description = description
    c.Category = category
    c.Difficulty = difficulty
    c.Visible = visible
    c.SQLEnabled = sqlEnabled
    c.SQLDatasetURL = sqlDatasetURL
    c.SQLSchemaHint = sqlSchemaHint
    return &c, nil
}
```

Update UpdateChallenge:

```go
func (db *DB) UpdateChallenge(id, name, description, category, difficulty string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string) error {
    query := `UPDATE challenges 
              SET name = ?, description = ?, category = ?, difficulty = ?, visible = ?,
                  sql_enabled = ?, sql_dataset_url = ?, sql_schema_hint = ?, updated_at = CURRENT_TIMESTAMP 
              WHERE id = ?`
    sqlEnabledInt := 0
    if sqlEnabled {
        sqlEnabledInt = 1
    }
    _, err := db.Exec(query, name, description, category, difficulty, visible, sqlEnabledInt, sqlDatasetURL, sqlSchemaHint, id)
    return err
}
```

**Step 4: Update admin template with SQL options**

File: `internal/views/templates/admin.html`

In the Create Challenge form, add after the visible checkbox:

```html
<div class="border-t border-dark-border pt-4 mt-4">
    <h4 class="text-sm font-medium text-gray-300 mb-3">SQL Playground Options</h4>
    <div class="flex items-center mb-3">
        <input
            type="checkbox"
            id="sql-enabled-create"
            name="sql_enabled"
            value="on"
            class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer">
        <label for="sql-enabled-create" class="ml-2 text-sm text-gray-300 cursor-pointer">Enable SQL Playground for this challenge</label>
    </div>
    <div>
        <label class="block text-sm font-medium text-gray-300 mb-1">Dataset URL (optional)</label>
        <input
            type="url"
            name="sql_dataset_url"
            placeholder="https://example.com/dataset.csv"
            class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
        <p class="text-xs text-gray-500 mt-1">URL to download dataset (CSV/JSON/Parquet). If empty, uses default CTF data.</p>
    </div>
    <div class="mt-3">
        <label class="block text-sm font-medium text-gray-300 mb-1">Schema Hint</label>
        <textarea
            name="sql_schema_hint"
            placeholder="-- Available tables:
-- users (id, name, email, created_at)
-- orders (id, user_id, amount, created_at)"
            rows="3"
            class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm font-mono"></textarea>
    </div>
</div>
```

**Step 5: Update challenge detail template with SQL support**

File: `internal/views/templates/challenge.html`

Add SQL playground interface when challenge has sql_enabled. This requires significant changes to integrate the DuckDB WASM editor.

**Step 6: Commit**

```bash
git add internal/database/migrations/ internal/models/models.go internal/database/queries.go internal/views/templates/admin.html
git commit -m "feat(challenges): add SQL Playground integration foundation

- Add sql_enabled, sql_dataset_url, sql_schema_hint columns to challenges
- Add expected_query_result column to questions
- Update Challenge and Question models
- Update queries to handle new fields
- Add SQL options to admin challenge forms

Part of SQL Playground for Challenges feature"
```

---

## Task 4: Public User Profiles Enhancement

**Description:** Ensure users can click on other users' profiles to view their public stats (already implemented but verify and enhance).

**Analysis:** The feature is already partially implemented:
- `/profile` - shows own profile (requires auth)
- `/users/{id}` - shows any user's profile (public)
- Both use the same template

The profile already shows stats, solved challenges, and recent activity. We just need to ensure clicking user names/links works correctly.

**Files:**
- Modify: `internal/views/templates/scoreboard.html` (add profile links)
- Modify: `internal/views/templates/teams.html` (add profile links)
- Modify: `internal/views/templates/challenge.html` (add profile links to solve messages)

**Step 1: Add profile links to scoreboard**

File: `internal/views/templates/scoreboard.html`

Look for user name display and wrap with link:

```html
<!-- Add link around user name -->
<a href="/users/{{.UserID}}" class="text-white hover:text-blue-400 font-medium">
    {{.UserName}}
</a>
```

**Step 2: Add profile links to teams page**

File: `internal/views/templates/teams.html`

In the members list, add links to user profiles.

**Step 3: Commit**

```bash
git add internal/views/templates/scoreboard.html internal/views/templates/teams.html
git commit -m "feat(ui): add profile links throughout the app

- Link user names on scoreboard to their public profiles
- Link team members to their profiles"
```

---

## Task 5: Dark/Light Theme Toggle

**Description:** Implement a theme toggle button with a new light theme.

**Files:**
- Modify: `internal/views/templates/base.html` (add theme toggle, light theme styles)
- Create: `internal/views/static/js/theme.js` (theme switching logic)
- Modify: `main.go` (serve theme.js if needed)

**Step 1: Add theme toggle to navigation**

File: `internal/views/templates/base.html`

1. Update the html tag to support theme classes:

```html
<!DOCTYPE html>
<html lang="en" class="dark">
```

2. Add theme toggle button in the navigation (before the user auth section, around line 64):

```html
<div class="flex items-center gap-4">
    <!-- Theme Toggle -->
    <button 
        @click="
            const html = document.documentElement;
            const isDark = html.classList.contains('dark');
            if (isDark) {
                html.classList.remove('dark');
                localStorage.setItem('theme', 'light');
            } else {
                html.classList.add('dark');
                localStorage.setItem('theme', 'dark');
            }
        "
        x-data="{ isDark: true }"
        x-init="isDark = document.documentElement.classList.contains('dark')"
        @theme-changed.window="isDark = document.documentElement.classList.contains('dark')"
        class="p-2 rounded-lg text-gray-400 hover:text-white hover:bg-dark-border transition"
        title="Toggle theme">
        <span x-show="isDark">☀️</span>
        <span x-show="!isDark">🌙</span>
    </button>
```

3. Add theme initialization script in the head section (after the style tag, before closing </head>):

```html
<script>
    // Initialize theme before page renders to prevent flash
    (function() {
        const savedTheme = localStorage.getItem('theme');
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        
        if (savedTheme === 'light') {
            document.documentElement.classList.remove('dark');
        } else if (savedTheme === 'dark' || (!savedTheme && prefersDark)) {
            document.documentElement.classList.add('dark');
        }
    })();
</script>
```

4. Update body styles to work with both themes:

```html
<body class="min-h-screen flex flex-col bg-gray-50 dark:bg-[#0f172a] text-gray-900 dark:text-[#e2e8f0] transition-colors duration-200">
```

5. Update navigation styles for light theme:

```html
<nav class="bg-white dark:bg-dark-surface border-b border-gray-200 dark:border-dark-border transition-colors duration-200">
```

6. Update all hardcoded dark colors to use dark: modifiers throughout the template.

**Step 2: Update Tailwind config to support light mode**

File: `internal/views/templates/base.html`

Update the Tailwind config script:

```javascript
<script>
    if (typeof tailwind !== 'undefined') {
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    colors: {
                        dark: {
                            bg: '#0f172a',
                            surface: '#1e293b',
                            border: '#334155',
                        }
                    }
                }
            }
        }
    }
</script>
```

**Step 3: Update all templates to support light theme**

This requires going through each template and adding `dark:` prefixes to existing colors, and adding light theme colors as defaults.

Key changes needed in each template:
- `challenges.html`: Change `bg-dark-surface` to `bg-white dark:bg-dark-surface`
- `challenge.html`: Same pattern for all dark colors
- `profile.html`: Same pattern
- `scoreboard.html`: Same pattern
- `teams.html`: Same pattern
- `admin.html`: Same pattern
- `index.html`: Same pattern
- All auth pages: Same pattern

**Step 4: Commit**

```bash
git add internal/views/templates/
git commit -m "feat(ui): implement dark/light theme toggle

- Add theme toggle button to navigation bar
- Add theme initialization script to prevent flash
- Update all templates with dark: prefixes for light theme support
- Persist theme preference in localStorage
- Respect system preference as default"
```

---

## Testing Checklist

Before marking complete, verify:

### User Management
- [ ] Admin can see Users tab
- [ ] User list displays correctly
- [ ] Promote/Demote works
- [ ] Delete user works
- [ ] Self-modification is prevented

### Challenge Completion
- [ ] Progress bar shows on challenge cards for logged-in users
- [ ] Completed challenges show green styling
- [ ] Completion percentage is accurate

### SQL Playground for Challenges
- [ ] Admin can enable SQL mode on challenges
- [ ] Dataset URL can be set
- [ ] Schema hint displays to users
- [ ] SQL submission works (if implementing full feature)

### Public Profiles
- [ ] Clicking user names goes to public profile
- [ ] Public profile shows correct stats
- [ ] Own profile shows full details

### Theme Toggle
- [ ] Toggle button switches theme
- [ ] Theme persists across page loads
- [ ] No flash on page load
- [ ] All pages look correct in both themes

---

## Documentation Updates

After implementation, update:

1. **CLAUDE.md** - Add new handlers to the handler table, update feature list
2. **API.md** - Document new admin endpoints
3. **FEATURES_IMPLEMENTATION.md** - Add new features to implemented list

---

## Version Summary

| Feature | Version | Status |
|---------|---------|--------|
| Initial | v0.2.1 | ✅ Complete |
| User Management + Challenge Completion | v0.3.0 | 🔄 Planned |
| SQL Playground for Challenges | v0.4.0 | 🔄 Planned |
| Public Profiles Enhancement | v0.4.0 | 🔄 Planned |
| Dark/Light Theme Toggle | v0.5.0 | 🔄 Planned |

---

**End of Implementation Plan**
