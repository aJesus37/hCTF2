# hCTF2 CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a full CLI to the hCTF2 binary using Cobra subcommands and charmbracelet for rich terminal output, covering both admin and participant workflows.

**Architecture:** The existing server logic moves into `cmd/serve.go` as a cobra `RunE`; `main.go` becomes a thin cobra dispatcher. A new `internal/client/` package wraps the existing REST API for CLI use. A new `internal/tui/` package holds charmbracelet components. A new `internal/config/` package manages `~/.config/hctf2/config.yaml`.

**Tech Stack:** `cobra` (subcommands), `huh` (interactive forms), `lipgloss` (styled tables), `glamour` (markdown), `bubbletea` (interactive browser), `golang.org/x/term` (TTY detection), `gopkg.in/yaml.v3` (config file).

---

## Task 1: Add dependencies

**Files:**
- Modify: `go.mod` / `go.sum` (via `go get`)

**Step 1: Add all new dependencies**

```bash
cd /home/jesus/Projects/hCTF2
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/huh@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/glamour@latest
go get github.com/charmbracelet/bubbletea@latest
go get golang.org/x/term@latest
go get gopkg.in/yaml.v3@latest
go mod tidy
```

**Step 2: Verify it still builds**

```bash
go build ./...
```
Expected: no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build(deps): add cobra, charmbracelet, yaml deps for CLI"
```

---

## Task 2: Config package

**Files:**
- Create: `internal/config/config.go`

**Step 1: Write the test**

Create `internal/config/config_test.go`:

```go
package config_test

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/ajesus37/hCTF2/internal/config"
)

func TestSaveLoad(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "config.yaml")
    t.Setenv("HCTF2_CONFIG", path)

    cfg := &config.Config{
        Server:       "http://localhost:8090",
        Token:        "tok123",
        TokenExpires: time.Now().Add(time.Hour).UTC().Truncate(time.Second),
    }
    if err := config.Save(cfg); err != nil {
        t.Fatal(err)
    }

    loaded, err := config.Load()
    if err != nil {
        t.Fatal(err)
    }
    if loaded.Server != cfg.Server {
        t.Errorf("server: got %q want %q", loaded.Server, cfg.Server)
    }
    if loaded.Token != cfg.Token {
        t.Errorf("token: got %q want %q", loaded.Token, cfg.Token)
    }
}

func TestLoadMissing(t *testing.T) {
    t.Setenv("HCTF2_CONFIG", "/tmp/hctf2-nonexistent-test.yaml")
    os.Remove("/tmp/hctf2-nonexistent-test.yaml")

    cfg, err := config.Load()
    if err != nil {
        t.Fatal("Load() should not error on missing file:", err)
    }
    if cfg.Server != "http://localhost:8090" {
        t.Errorf("expected default server, got %q", cfg.Server)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -v
```
Expected: compile error (package doesn't exist yet).

**Step 3: Implement**

Create `internal/config/config.go`:

```go
package config

import (
    "os"
    "path/filepath"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Server       string    `yaml:"server"`
    Token        string    `yaml:"token,omitempty"`
    TokenExpires time.Time `yaml:"token_expires,omitempty"`
}

func path() string {
    if p := os.Getenv("HCTF2_CONFIG"); p != "" {
        return p
    }
    dir, _ := os.UserConfigDir()
    return filepath.Join(dir, "hctf2", "config.yaml")
}

func Load() (*Config, error) {
    cfg := &Config{Server: "http://localhost:8090"}
    data, err := os.ReadFile(path())
    if os.IsNotExist(err) {
        return cfg, nil
    }
    if err != nil {
        return nil, err
    }
    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}

func Save(cfg *Config) error {
    p := path()
    if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
        return err
    }
    data, err := yaml.Marshal(cfg)
    if err != nil {
        return err
    }
    return os.WriteFile(p, data, 0600)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -v
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(cli): add config package with Load/Save"
```

---

## Task 3: HTTP client package

**Files:**
- Create: `internal/client/client.go`
- Create: `internal/client/auth.go`
- Create: `internal/client/challenges.go`
- Create: `internal/client/teams.go`
- Create: `internal/client/competitions.go`
- Create: `internal/client/users.go`

**Step 1: Write the test (client base)**

Create `internal/client/client_test.go`:

```go
package client_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/ajesus37/hCTF2/internal/client"
)

func TestDoSetsAuthCookie(t *testing.T) {
    var gotCookie string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        c, _ := r.Cookie("auth_token")
        if c != nil {
            gotCookie = c.Value
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{}`))
    }))
    defer srv.Close()

    c := client.New(srv.URL, "mytoken")
    req, _ := http.NewRequest("GET", srv.URL+"/api/challenges", nil)
    resp, err := c.Do(req)
    if err != nil {
        t.Fatal(err)
    }
    resp.Body.Close()
    if gotCookie != "mytoken" {
        t.Errorf("expected cookie auth_token=mytoken, got %q", gotCookie)
    }
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/client/... -v
```
Expected: compile error.

**Step 3: Implement base client**

Create `internal/client/client.go`:

```go
package client

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    ServerURL  string
    Token      string
    httpClient *http.Client
}

func New(serverURL, token string) *Client {
    return &Client{
        ServerURL:  serverURL,
        Token:      token,
        httpClient: &http.Client{Timeout: 15 * time.Second},
    }
}

// Do executes a request, injecting the auth cookie if a token is set.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
    if c.Token != "" {
        req.AddCookie(&http.Cookie{Name: "auth_token", Value: c.Token})
    }
    return c.httpClient.Do(req)
}

// decodeJSON decodes a JSON response body into v.
func decodeJSON(resp *http.Response, v any) error {
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusForbidden {
        return fmt.Errorf("admin privileges required")
    }
    if resp.StatusCode == http.StatusUnauthorized {
        return fmt.Errorf("not authenticated — run 'hctf2 login'")
    }
    if resp.StatusCode >= 400 {
        var e struct{ Error string `json:"error"` }
        _ = json.NewDecoder(resp.Body).Decode(&e)
        if e.Error != "" {
            return fmt.Errorf("server error: %s", e.Error)
        }
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return json.NewDecoder(resp.Body).Decode(v)
}
```

**Step 4: Implement auth.go**

Create `internal/client/auth.go`:

```go
package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type LoginResponse struct {
    Token   string `json:"token"`
    UserID  string `json:"user_id"`
    IsAdmin bool   `json:"is_admin"`
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
    body, _ := json.Marshal(map[string]string{"email": email, "password": password})
    req, _ := http.NewRequest("POST", c.ServerURL+"/api/auth/login", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("connection failed: %w", err)
    }
    var lr LoginResponse
    if err := decodeJSON(resp, &lr); err != nil {
        return nil, err
    }
    return &lr, nil
}
```

**Step 5: Implement challenges.go**

Create `internal/client/challenges.go`:

```go
package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type Challenge struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    Category    string `json:"category"`
    Difficulty  string `json:"difficulty"`
    Points      int    `json:"points"`
    Description string `json:"description"`
    Solved      bool   `json:"solved"`
}

type Question struct {
    ID    string `json:"id"`
    Title string `json:"title"`
    Body  string `json:"body"`
}

type SubmitResult struct {
    Correct bool   `json:"correct"`
    Message string `json:"message"`
    Points  int    `json:"points"`
}

func (c *Client) ListChallenges() ([]Challenge, error) {
    req, _ := http.NewRequest("GET", c.ServerURL+"/api/challenges", nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out []Challenge
    return out, decodeJSON(resp, &out)
}

func (c *Client) GetChallenge(id string) (*Challenge, error) {
    req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/challenges/%s", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out Challenge
    return &out, decodeJSON(resp, &out)
}

func (c *Client) SubmitFlag(questionID, flag string) (*SubmitResult, error) {
    body, _ := json.Marshal(map[string]string{"flag": flag})
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/questions/%s/submit", c.ServerURL, questionID), bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out SubmitResult
    return &out, decodeJSON(resp, &out)
}

func (c *Client) CreateChallenge(title, category, difficulty, description string, points int) (*Challenge, error) {
    body, _ := json.Marshal(map[string]any{
        "title": title, "category": category, "difficulty": difficulty,
        "description": description, "points": points,
    })
    req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/challenges", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out Challenge
    return &out, decodeJSON(resp, &out)
}

func (c *Client) DeleteChallenge(id string) error {
    req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/challenges/%s", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}
```

**Step 6: Implement teams.go**

Create `internal/client/teams.go`:

```go
package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type Team struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    Score      int    `json:"score"`
    MemberCount int   `json:"member_count"`
    InviteCode string `json:"invite_code,omitempty"`
}

func (c *Client) ListTeams() ([]Team, error) {
    req, _ := http.NewRequest("GET", c.ServerURL+"/api/teams", nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out []Team
    return out, decodeJSON(resp, &out)
}

func (c *Client) GetTeam(id string) (*Team, error) {
    req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/teams/%s", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out Team
    return &out, decodeJSON(resp, &out)
}

func (c *Client) CreateTeam(name string) (*Team, error) {
    body, _ := json.Marshal(map[string]string{"name": name})
    req, _ := http.NewRequest("POST", c.ServerURL+"/api/teams", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out Team
    return &out, decodeJSON(resp, &out)
}

func (c *Client) JoinTeam(inviteCode string) error {
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/teams/join/%s", c.ServerURL, inviteCode), nil)
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}
```

**Step 7: Implement competitions.go**

Create `internal/client/competitions.go`:

```go
package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type Competition struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}

func (c *Client) ListCompetitions() ([]Competition, error) {
    req, _ := http.NewRequest("GET", c.ServerURL+"/api/competitions", nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out []Competition
    return out, decodeJSON(resp, &out)
}

func (c *Client) CreateCompetition(name string) (*Competition, error) {
    body, _ := json.Marshal(map[string]string{"name": name})
    req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/competitions", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out Competition
    return &out, decodeJSON(resp, &out)
}

func (c *Client) ForceStartCompetition(id string) error {
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%s/force-start", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}

func (c *Client) ForceEndCompetition(id string) error {
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%s/force-end", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}
```

**Step 8: Implement users.go**

Create `internal/client/users.go`:

```go
package client

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type User struct {
    ID      string `json:"id"`
    Email   string `json:"email"`
    Name    string `json:"name"`
    IsAdmin bool   `json:"is_admin"`
}

func (c *Client) ListUsers() ([]User, error) {
    req, _ := http.NewRequest("GET", c.ServerURL+"/api/admin/users", nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, err
    }
    var out []User
    return out, decodeJSON(resp, &out)
}

func (c *Client) PromoteUser(id string, admin bool) error {
    body, _ := json.Marshal(map[string]bool{"is_admin": admin})
    req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/admin/users/%s/admin", c.ServerURL, id), jsonBody(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}

func (c *Client) DeleteUser(id string) error {
    req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/users/%s", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("server returned %d", resp.StatusCode)
    }
    return nil
}
```

Add helper to `client.go` (add at bottom):

```go
import "bytes"

func jsonBody(data []byte) *bytes.Reader {
    return bytes.NewReader(data)
}
```

**Step 9: Run tests**

```bash
go test ./internal/client/... -v
```
Expected: PASS (base client test).

**Step 10: Verify build**

```bash
go build ./...
```

**Step 11: Commit**

```bash
git add internal/client/
git commit -m "feat(cli): add HTTP client package for all API domains"
```

---

## Task 4: TUI package (table + theme)

**Files:**
- Create: `internal/tui/theme.go`
- Create: `internal/tui/table.go`

**Step 1: Implement theme**

Create `internal/tui/theme.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
    HeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
    CellStyle    = lipgloss.NewStyle().PaddingRight(2)
    SolvedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
    ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
    MutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
    SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)
```

**Step 2: Implement table renderer**

Create `internal/tui/table.go`:

```go
package tui

import (
    "fmt"
    "io"
    "strings"
)

// Column defines a table column with a header and width.
type Column struct {
    Header string
    Width  int
}

// PrintTable writes a lipgloss-styled table to w.
func PrintTable(w io.Writer, cols []Column, rows [][]string) {
    // Header
    var headers []string
    for _, c := range cols {
        headers = append(headers, HeaderStyle.Width(c.Width).Render(c.Header))
    }
    fmt.Fprintln(w, strings.Join(headers, ""))

    // Separator
    var sep []string
    for _, c := range cols {
        sep = append(sep, MutedStyle.Render(strings.Repeat("─", c.Width)))
    }
    fmt.Fprintln(w, strings.Join(sep, ""))

    // Rows
    for _, row := range rows {
        var cells []string
        for i, c := range cols {
            val := ""
            if i < len(row) {
                val = row[i]
            }
            cells = append(cells, CellStyle.Width(c.Width).Render(val))
        }
        fmt.Fprintln(w, strings.Join(cells, ""))
    }
}
```

**Step 3: Verify build**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/tui/
git commit -m "feat(cli): add TUI table and theme helpers"
```

---

## Task 5: Cobra root + serve subcommand

**Files:**
- Modify: `main.go` — replace `flag`-based entry point with cobra dispatch
- Create: `cmd/root.go`
- Create: `cmd/serve.go`

**Step 1: Create cmd/root.go**

```go
package cmd

import (
    "os"

    "github.com/spf13/cobra"
)

var (
    serverOverride string
    jsonOutput     bool
    quietOutput    bool
)

var rootCmd = &cobra.Command{
    Use:   "hctf2",
    Short: "hCTF2 — self-hosted CTF platform",
    Long:  "hCTF2 is a self-hosted CTF platform. Run 'hctf2 serve' to start the server.",
}

func Execute(version string) {
    rootCmd.Version = version
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

func init() {
    rootCmd.PersistentFlags().StringVar(&serverOverride, "server", "", "Server URL (overrides config)")
    rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
    rootCmd.PersistentFlags().BoolVar(&quietOutput, "quiet", false, "Minimal output")
}
```

**Step 2: Create cmd/serve.go**

Move all flag declarations and server startup from `main()` into this file:

```go
package cmd

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "runtime"
    "syscall"
    "time"

    "github.com/spf13/cobra"
    // ... same imports as current main.go server block
)

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the hCTF2 web server",
    RunE:  runServe,
}

var (
    servePort             int
    serveDB               string
    serveAdminEmail       string
    serveAdminPass        string
    serveMotd             string
    servePrometheus       bool
    serveOtlpEndpoint     string
    serveSmtpHost         string
    serveSmtpPort         int
    serveSmtpFrom         string
    serveSmtpUser         string
    serveSmtpPass         string
    serveBaseURL          string
    serveJWTSecret        string
    serveDev              bool
    serveCorsOrigins      string
    serveRateLimit        int
    serveUploadDir        string
)

func init() {
    rootCmd.AddCommand(serveCmd)
    f := serveCmd.Flags()
    f.IntVar(&servePort, "port", 8090, "Server port")
    f.StringVar(&serveDB, "db", "./hctf2.db", "Database path")
    f.StringVar(&serveAdminEmail, "admin-email", "", "Admin email for first-time setup")
    f.StringVar(&serveAdminPass, "admin-password", "", "Admin password for first-time setup")
    f.StringVar(&serveMotd, "motd", "", "Message of the Day")
    f.BoolVar(&servePrometheus, "metrics", false, "Enable Prometheus /metrics endpoint")
    f.StringVar(&serveOtlpEndpoint, "otel-otlp-endpoint", "", "OTLP exporter endpoint")
    f.StringVar(&serveSmtpHost, "smtp-host", "", "SMTP server host")
    f.IntVar(&serveSmtpPort, "smtp-port", 587, "SMTP server port")
    f.StringVar(&serveSmtpFrom, "smtp-from", "", "SMTP from address")
    f.StringVar(&serveSmtpUser, "smtp-user", "", "SMTP username")
    f.StringVar(&serveSmtpPass, "smtp-password", "", "SMTP password")
    f.StringVar(&serveBaseURL, "base-url", "http://localhost:8090", "Base URL for email links")
    f.StringVar(&serveJWTSecret, "jwt-secret", getEnv("JWT_SECRET", ""), "JWT signing secret")
    f.BoolVar(&serveDev, "dev", false, "Development mode")
    f.StringVar(&serveCorsOrigins, "cors-origins", getEnv("CORS_ORIGINS", ""), "Allowed CORS origins")
    f.IntVar(&serveRateLimit, "submission-rate-limit", 5, "Max flag submissions per minute per user")
    f.StringVar(&serveUploadDir, "upload-dir", "./uploads", "Directory for file uploads")
}

func runServe(cmd *cobra.Command, args []string) error {
    // Move the entire body of the current main() here, using serve* vars instead of flag vars.
    // ... (copy server startup logic from main.go)
    return nil
}
```

**Step 3: Update main.go**

Replace the entire `main()` body with:

```go
package main

import "github.com/ajesus37/hCTF2/cmd"

var version = "dev"

func main() {
    cmd.Execute(version)
}
```

Keep the `//go:generate` and embed directives at the top — move them to `cmd/serve.go` since they are only needed by the server.

**Step 4: Verify it builds and serve still works**

```bash
go build -o hctf2 . && ./hctf2 serve --help
```
Expected: shows serve flags.

```bash
./hctf2 --help
```
Expected: shows subcommands including `serve`.

**Step 5: Run existing tests**

```bash
go test ./...
```
Expected: all pass.

**Step 6: Commit**

```bash
git add cmd/ main.go
git commit -m "feat(cli): migrate server to cobra serve subcommand"
```

---

## Task 6: login / logout / status commands

**Files:**
- Create: `cmd/auth.go`

**Step 1: Implement**

Create `cmd/auth.go`:

```go
package cmd

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/ajesus37/hCTF2/internal/client"
    "github.com/ajesus37/hCTF2/internal/config"
    "github.com/charmbracelet/huh"
    "github.com/spf13/cobra"
    "golang.org/x/term"
)

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Log in to an hCTF2 server",
    RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
    Use:   "logout",
    Short: "Clear saved credentials",
    RunE:  runLogout,
}

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show current server and auth status",
    RunE:  runStatus,
}

var (
    loginEmail    string
    loginPassword string
    loginServer   string
)

func init() {
    rootCmd.AddCommand(loginCmd)
    rootCmd.AddCommand(logoutCmd)
    rootCmd.AddCommand(statusCmd)
    loginCmd.Flags().StringVar(&loginEmail, "email", "", "Email address")
    loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password")
    loginCmd.Flags().StringVar(&loginServer, "server", "", "Server URL (e.g. http://localhost:8090)")
}

func runLogin(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()

    server := loginServer
    if server == "" && serverOverride != "" {
        server = serverOverride
    }
    if server == "" {
        server = cfg.Server
    }

    email := loginEmail
    password := loginPassword

    // Interactive prompts when running in a TTY and values missing
    if term.IsTerminal(int(os.Stdin.Fd())) && (email == "" || password == "" || loginServer == "") {
        var fields []huh.Field
        if loginServer == "" {
            fields = append(fields, huh.NewInput().Title("Server URL").Value(&server))
        }
        if email == "" {
            fields = append(fields, huh.NewInput().Title("Email").Value(&email))
        }
        if password == "" {
            fields = append(fields, huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password))
        }
        if len(fields) > 0 {
            if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
                return err
            }
        }
    }

    if email == "" || password == "" {
        return fmt.Errorf("--email and --password required")
    }

    c := client.New(server, "")
    lr, err := c.Login(email, password)
    if err != nil {
        return err
    }

    cfg.Server = server
    cfg.Token = lr.Token
    cfg.TokenExpires = jwtExpiry(lr.Token)
    if err := config.Save(cfg); err != nil {
        return err
    }

    if !quietOutput {
        fmt.Fprintf(os.Stdout, "Logged in to %s\n", server)
    }
    return nil
}

func runLogout(_ *cobra.Command, _ []string) error {
    cfg, _ := config.Load()
    cfg.Token = ""
    cfg.TokenExpires = time.Time{}
    if err := config.Save(cfg); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintln(os.Stdout, "Logged out")
    }
    return nil
}

func runStatus(_ *cobra.Command, _ []string) error {
    cfg, _ := config.Load()
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(cfg)
    }
    fmt.Fprintf(os.Stdout, "Server:  %s\n", cfg.Server)
    if cfg.Token == "" {
        fmt.Fprintln(os.Stdout, "Auth:    not logged in")
    } else if time.Now().After(cfg.TokenExpires) {
        fmt.Fprintln(os.Stdout, "Auth:    session expired — run 'hctf2 login'")
    } else {
        user := jwtSubject(cfg.Token)
        fmt.Fprintf(os.Stdout, "Auth:    %s (expires %s)\n", user, cfg.TokenExpires.Format(time.RFC3339))
    }
    return nil
}

// jwtExpiry extracts exp claim from a JWT without verifying signature.
func jwtExpiry(token string) time.Time {
    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return time.Now().Add(24 * time.Hour)
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return time.Now().Add(24 * time.Hour)
    }
    var claims struct{ Exp int64 `json:"exp"` }
    _ = json.Unmarshal(payload, &claims)
    if claims.Exp == 0 {
        return time.Now().Add(24 * time.Hour)
    }
    return time.Unix(claims.Exp, 0)
}

// jwtSubject extracts sub claim (email) from a JWT without verifying.
func jwtSubject(token string) string {
    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return "unknown"
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return "unknown"
    }
    var claims struct{ Sub string `json:"sub"` }
    _ = json.Unmarshal(payload, &claims)
    return claims.Sub
}
```

**Step 2: Verify build**

```bash
go build ./... && ./hctf2 login --help && ./hctf2 status --help
```

**Step 3: Commit**

```bash
git add cmd/auth.go
git commit -m "feat(cli): add login, logout, status commands"
```

---

## Task 7: challenge commands

**Files:**
- Create: `cmd/challenge.go`

**Step 1: Implement**

Create `cmd/challenge.go`:

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "strconv"

    "github.com/ajesus37/hCTF2/internal/client"
    "github.com/ajesus37/hCTF2/internal/config"
    "github.com/ajesus37/hCTF2/internal/tui"
    "github.com/charmbracelet/glamour"
    "github.com/charmbracelet/huh"
    "github.com/spf13/cobra"
    "golang.org/x/term"
)

var challengeCmd = &cobra.Command{Use: "challenge", Short: "Manage and browse challenges", Aliases: []string{"ch"}}
var challengeListCmd = &cobra.Command{Use: "list", Short: "List all challenges", RunE: runChallengeList}
var challengeGetCmd = &cobra.Command{Use: "get <id>", Short: "Show challenge details", Args: cobra.ExactArgs(1), RunE: runChallengeGet}
var challengeCreateCmd = &cobra.Command{Use: "create", Short: "Create a challenge (admin)", RunE: runChallengeCreate}
var challengeDeleteCmd = &cobra.Command{Use: "delete <id>", Short: "Delete a challenge (admin)", Args: cobra.ExactArgs(1), RunE: runChallengeDelete}

var (
    createTitle       string
    createCategory    string
    createDifficulty  string
    createDescription string
    createPoints      int
)

func init() {
    rootCmd.AddCommand(challengeCmd)
    challengeCmd.AddCommand(challengeListCmd, challengeGetCmd, challengeCreateCmd, challengeDeleteCmd)
    challengeCreateCmd.Flags().StringVar(&createTitle, "title", "", "Challenge title")
    challengeCreateCmd.Flags().StringVar(&createCategory, "category", "", "Category")
    challengeCreateCmd.Flags().StringVar(&createDifficulty, "difficulty", "", "Difficulty")
    challengeCreateCmd.Flags().StringVar(&createDescription, "description", "", "Description (markdown)")
    challengeCreateCmd.Flags().IntVar(&createPoints, "points", 100, "Point value")
}

func newClient() (*client.Client, error) {
    cfg, err := config.Load()
    if err != nil {
        return nil, err
    }
    if serverOverride != "" {
        cfg.Server = serverOverride
    }
    if cfg.Token == "" {
        return nil, fmt.Errorf("not logged in — run 'hctf2 login'")
    }
    return client.New(cfg.Server, cfg.Token), nil
}

func runChallengeList(_ *cobra.Command, _ []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    challenges, err := c.ListChallenges()
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(challenges)
    }
    cols := []tui.Column{
        {Header: "ID", Width: 10},
        {Header: "TITLE", Width: 30},
        {Header: "CATEGORY", Width: 15},
        {Header: "DIFF", Width: 12},
        {Header: "PTS", Width: 6},
        {Header: "SOLVED", Width: 7},
    }
    var rows [][]string
    for _, ch := range challenges {
        solved := ""
        if ch.Solved {
            solved = tui.SolvedStyle.Render("✓")
        }
        rows = append(rows, []string{
            ch.ID[:8] + "...",
            ch.Title,
            ch.Category,
            ch.Difficulty,
            strconv.Itoa(ch.Points),
            solved,
        })
    }
    tui.PrintTable(os.Stdout, cols, rows)
    return nil
}

func runChallengeGet(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    ch, err := c.GetChallenge(args[0])
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(ch)
    }
    fmt.Fprintf(os.Stdout, "%s  %s  [%s / %s]  %d pts\n\n",
        tui.HeaderStyle.Render(ch.Title), tui.MutedStyle.Render(ch.ID),
        ch.Category, ch.Difficulty, ch.Points)
    if ch.Description != "" {
        r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
        out, _ := r.Render(ch.Description)
        fmt.Fprint(os.Stdout, out)
    }
    return nil
}

func runChallengeCreate(_ *cobra.Command, _ []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if term.IsTerminal(int(os.Stdin.Fd())) && (createTitle == "" || createCategory == "") {
        _ = huh.NewForm(huh.NewGroup(
            huh.NewInput().Title("Title").Value(&createTitle),
            huh.NewInput().Title("Category").Value(&createCategory),
            huh.NewInput().Title("Difficulty").Value(&createDifficulty),
            huh.NewInput().Title("Points").Value(func() *string { s := strconv.Itoa(createPoints); return &s }()),
        )).Run()
    }
    ch, err := c.CreateChallenge(createTitle, createCategory, createDifficulty, createDescription, createPoints)
    if err != nil {
        return err
    }
    if quietOutput {
        fmt.Fprintln(os.Stdout, ch.ID)
        return nil
    }
    fmt.Fprintf(os.Stdout, "Created challenge %s (%s)\n", ch.Title, ch.ID)
    return nil
}

func runChallengeDelete(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.DeleteChallenge(args[0]); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "Deleted %s\n", args[0])
    }
    return nil
}
```

**Step 2: Verify build and help**

```bash
go build ./... && ./hctf2 challenge --help && ./hctf2 challenge list --help
```

**Step 3: Commit**

```bash
git add cmd/challenge.go
git commit -m "feat(cli): add challenge list/get/create/delete commands"
```

---

## Task 8: flag, hint, team, competition, user commands

**Files:**
- Create: `cmd/flag.go`
- Create: `cmd/team.go`
- Create: `cmd/competition.go`
- Create: `cmd/user.go`

**Step 1: Create cmd/flag.go**

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/ajesus37/hCTF2/internal/tui"
    "github.com/spf13/cobra"
)

var flagCmd = &cobra.Command{Use: "flag", Short: "Flag submission"}
var flagSubmitCmd = &cobra.Command{
    Use:   "submit <question-id> <flag>",
    Short: "Submit a flag for a question",
    Args:  cobra.ExactArgs(2),
    RunE:  runFlagSubmit,
}

func init() {
    rootCmd.AddCommand(flagCmd)
    flagCmd.AddCommand(flagSubmitCmd)
}

func runFlagSubmit(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    result, err := c.SubmitFlag(args[0], args[1])
    if err != nil {
        return err
    }
    if result.Correct {
        fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render("Correct! +"+fmt.Sprint(result.Points)+" pts"))
    } else {
        fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("Incorrect flag"))
    }
    return nil
}
```

**Step 2: Create cmd/team.go**

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "strconv"

    "github.com/ajesus37/hCTF2/internal/tui"
    "github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{Use: "team", Short: "Team management"}
var teamListCmd = &cobra.Command{Use: "list", Short: "List all teams", RunE: runTeamList}
var teamGetCmd = &cobra.Command{Use: "get <id>", Short: "Show team details", Args: cobra.ExactArgs(1), RunE: runTeamGet}
var teamCreateCmd = &cobra.Command{Use: "create <name>", Short: "Create a team", Args: cobra.ExactArgs(1), RunE: runTeamCreate}
var teamJoinCmd = &cobra.Command{Use: "join <invite-code>", Short: "Join a team by invite code", Args: cobra.ExactArgs(1), RunE: runTeamJoin}

func init() {
    rootCmd.AddCommand(teamCmd)
    teamCmd.AddCommand(teamListCmd, teamGetCmd, teamCreateCmd, teamJoinCmd)
}

func runTeamList(_ *cobra.Command, _ []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    teams, err := c.ListTeams()
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(teams)
    }
    cols := []tui.Column{{Header: "ID", Width: 10}, {Header: "NAME", Width: 25}, {Header: "SCORE", Width: 8}, {Header: "MEMBERS", Width: 9}}
    var rows [][]string
    for _, t := range teams {
        rows = append(rows, []string{t.ID[:8] + "...", t.Name, strconv.Itoa(t.Score), strconv.Itoa(t.MemberCount)})
    }
    tui.PrintTable(os.Stdout, cols, rows)
    return nil
}

func runTeamGet(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    t, err := c.GetTeam(args[0])
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(t)
    }
    fmt.Fprintf(os.Stdout, "Name:    %s\nID:      %s\nScore:   %d\nMembers: %d\n",
        t.Name, t.ID, t.Score, t.MemberCount)
    if t.InviteCode != "" {
        fmt.Fprintf(os.Stdout, "Invite:  %s\n", t.InviteCode)
    }
    return nil
}

func runTeamCreate(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    t, err := c.CreateTeam(args[0])
    if err != nil {
        return err
    }
    if quietOutput {
        fmt.Fprintln(os.Stdout, t.ID)
        return nil
    }
    fmt.Fprintf(os.Stdout, "Created team %q (%s)\n", t.Name, t.ID)
    return nil
}

func runTeamJoin(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.JoinTeam(args[0]); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintln(os.Stdout, "Joined team")
    }
    return nil
}
```

**Step 3: Create cmd/competition.go**

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/ajesus37/hCTF2/internal/tui"
    "github.com/spf13/cobra"
)

var competitionCmd = &cobra.Command{Use: "competition", Short: "Competition management", Aliases: []string{"comp"}}
var compListCmd = &cobra.Command{Use: "list", Short: "List competitions", RunE: runCompList}
var compCreateCmd = &cobra.Command{Use: "create <name>", Short: "Create a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompCreate}
var compStartCmd = &cobra.Command{Use: "start <id>", Short: "Force-start a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompStart}
var compEndCmd = &cobra.Command{Use: "end <id>", Short: "Force-end a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompEnd}

func init() {
    rootCmd.AddCommand(competitionCmd)
    competitionCmd.AddCommand(compListCmd, compCreateCmd, compStartCmd, compEndCmd)
}

func runCompList(_ *cobra.Command, _ []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    comps, err := c.ListCompetitions()
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(comps)
    }
    cols := []tui.Column{{Header: "ID", Width: 10}, {Header: "NAME", Width: 30}, {Header: "STATUS", Width: 12}}
    var rows [][]string
    for _, co := range comps {
        rows = append(rows, []string{co.ID[:8] + "...", co.Name, co.Status})
    }
    tui.PrintTable(os.Stdout, cols, rows)
    return nil
}

func runCompCreate(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    co, err := c.CreateCompetition(args[0])
    if err != nil {
        return err
    }
    if quietOutput {
        fmt.Fprintln(os.Stdout, co.ID)
        return nil
    }
    fmt.Fprintf(os.Stdout, "Created competition %q (%s)\n", co.Name, co.ID)
    return nil
}

func runCompStart(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.ForceStartCompetition(args[0]); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "Started competition %s\n", args[0])
    }
    return nil
}

func runCompEnd(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.ForceEndCompetition(args[0]); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "Ended competition %s\n", args[0])
    }
    return nil
}
```

**Step 4: Create cmd/user.go**

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/ajesus37/hCTF2/internal/tui"
    "github.com/spf13/cobra"
)

var userCmd = &cobra.Command{Use: "user", Short: "User management (admin only)"}
var userListCmd = &cobra.Command{Use: "list", Short: "List all users", RunE: runUserList}
var userPromoteCmd = &cobra.Command{Use: "promote <id>", Short: "Grant admin to user", Args: cobra.ExactArgs(1), RunE: runUserPromote}
var userDemoteCmd = &cobra.Command{Use: "demote <id>", Short: "Revoke admin from user", Args: cobra.ExactArgs(1), RunE: runUserDemote}
var userDeleteCmd = &cobra.Command{Use: "delete <id>", Short: "Delete a user", Args: cobra.ExactArgs(1), RunE: runUserDelete}

func init() {
    rootCmd.AddCommand(userCmd)
    userCmd.AddCommand(userListCmd, userPromoteCmd, userDemoteCmd, userDeleteCmd)
}

func runUserList(_ *cobra.Command, _ []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    users, err := c.ListUsers()
    if err != nil {
        return err
    }
    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(users)
    }
    cols := []tui.Column{{Header: "ID", Width: 10}, {Header: "EMAIL", Width: 30}, {Header: "NAME", Width: 20}, {Header: "ADMIN", Width: 6}}
    var rows [][]string
    for _, u := range users {
        admin := ""
        if u.IsAdmin {
            admin = tui.SolvedStyle.Render("✓")
        }
        rows = append(rows, []string{u.ID[:8] + "...", u.Email, u.Name, admin})
    }
    tui.PrintTable(os.Stdout, cols, rows)
    return nil
}

func runUserPromote(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.PromoteUser(args[0], true); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "User %s promoted to admin\n", args[0])
    }
    return nil
}

func runUserDemote(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.PromoteUser(args[0], false); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "User %s demoted\n", args[0])
    }
    return nil
}

func runUserDelete(_ *cobra.Command, args []string) error {
    c, err := newClient()
    if err != nil {
        return err
    }
    if err := c.DeleteUser(args[0]); err != nil {
        return err
    }
    if !quietOutput {
        fmt.Fprintf(os.Stdout, "Deleted user %s\n", args[0])
    }
    return nil
}
```

**Step 5: Verify build**

```bash
go build ./...
./hctf2 --help          # should show all subcommands
./hctf2 user --help
./hctf2 flag --help
```

**Step 6: Run all tests**

```bash
go test ./...
```

**Step 7: Commit**

```bash
git add cmd/flag.go cmd/team.go cmd/competition.go cmd/user.go
git commit -m "feat(cli): add flag submit, team, competition, user commands"
```

---

## Task 9: challenge browse (bubbletea)

**Files:**
- Create: `internal/tui/browse.go`
- Modify: `cmd/challenge.go` — add `browse` subcommand

**Step 1: Implement bubbletea browser**

Create `internal/tui/browse.go`:

```go
package tui

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type Challenge struct {
    ID       string
    Title    string
    Category string
    Points   int
    Solved   bool
}

type BrowseModel struct {
    challenges []Challenge
    filtered   []Challenge
    cursor     int
    filter     string
    filtering  bool
    selected   *Challenge
    quit       bool
}

func NewBrowseModel(challenges []Challenge) BrowseModel {
    return BrowseModel{challenges: challenges, filtered: challenges}
}

func (m BrowseModel) Init() tea.Cmd { return nil }

func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.filtering {
            switch msg.String() {
            case "enter", "esc":
                m.filtering = false
            case "backspace":
                if len(m.filter) > 0 {
                    m.filter = m.filter[:len(m.filter)-1]
                    m.applyFilter()
                }
            default:
                m.filter += msg.String()
                m.applyFilter()
            }
            return m, nil
        }
        switch msg.String() {
        case "q", "ctrl+c":
            m.quit = true
            return m, tea.Quit
        case "/":
            m.filtering = true
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.filtered)-1 {
                m.cursor++
            }
        case "enter":
            if len(m.filtered) > 0 {
                ch := m.filtered[m.cursor]
                m.selected = &ch
                return m, tea.Quit
            }
        }
    }
    return m, nil
}

func (m *BrowseModel) applyFilter() {
    m.filtered = nil
    for _, ch := range m.challenges {
        if strings.Contains(strings.ToLower(ch.Title), strings.ToLower(m.filter)) ||
            strings.Contains(strings.ToLower(ch.Category), strings.ToLower(m.filter)) {
            m.filtered = append(m.filtered, ch)
        }
    }
    m.cursor = 0
}

func (m BrowseModel) View() string {
    var sb strings.Builder
    sb.WriteString(HeaderStyle.Render("Challenge Browser") + "\n")
    if m.filtering {
        sb.WriteString(fmt.Sprintf("Filter: %s█\n\n", m.filter))
    } else {
        sb.WriteString(MutedStyle.Render("↑/↓ navigate  / filter  enter select  q quit") + "\n\n")
    }
    for i, ch := range m.filtered {
        cursor := "  "
        if i == m.cursor {
            cursor = "> "
        }
        solved := ""
        if ch.Solved {
            solved = SolvedStyle.Render(" ✓")
        }
        line := fmt.Sprintf("%s%-28s %-14s %4dpts%s", cursor, ch.Title, ch.Category, ch.Points, solved)
        if i == m.cursor {
            sb.WriteString(lipgloss.NewStyle().Bold(true).Render(line) + "\n")
        } else {
            sb.WriteString(line + "\n")
        }
    }
    return sb.String()
}

// Run starts the browser and returns the selected challenge ID, or "" if none.
func RunBrowser(challenges []Challenge) (string, error) {
    m := NewBrowseModel(challenges)
    p := tea.NewProgram(m)
    result, err := p.Run()
    if err != nil {
        return "", err
    }
    final := result.(BrowseModel)
    if final.selected != nil {
        return final.selected.ID, nil
    }
    return "", nil
}
```

**Step 2: Add browse subcommand to cmd/challenge.go**

Add to the `init()` block: `challengeCmd.AddCommand(challengeBrowseCmd)`

Add command and handler:

```go
var challengeBrowseCmd = &cobra.Command{
    Use:   "browse",
    Short: "Interactively browse and select challenges",
    RunE:  runChallengeBrowse,
}

func runChallengeBrowse(_ *cobra.Command, _ []string) error {
    if !term.IsTerminal(int(os.Stdin.Fd())) {
        return fmt.Errorf("browse requires an interactive terminal")
    }
    c, err := newClient()
    if err != nil {
        return err
    }
    challenges, err := c.ListChallenges()
    if err != nil {
        return err
    }
    var tuiChallenges []tui.Challenge
    for _, ch := range challenges {
        tuiChallenges = append(tuiChallenges, tui.Challenge{
            ID: ch.ID, Title: ch.Title, Category: ch.Category,
            Points: ch.Points, Solved: ch.Solved,
        })
    }
    id, err := tui.RunBrowser(tuiChallenges)
    if err != nil {
        return err
    }
    if id == "" {
        return nil
    }
    // Show detail of selected challenge
    return runChallengeGet(nil, []string{id})
}
```

**Step 3: Verify build**

```bash
go build ./... && ./hctf2 challenge browse --help
```

**Step 4: Commit**

```bash
git add internal/tui/browse.go cmd/challenge.go
git commit -m "feat(cli): add interactive challenge browser with bubbletea"
```

---

## Task 10: Wire version/info into cobra + final validation

**Files:**
- Modify: `cmd/root.go` — add `version` and `info` subcommands
- Modify: `cmd/serve.go` — remove `--version`/`--info` flags (now subcommands)

**Step 1: Add version/info commands to cmd/root.go**

```go
import "runtime"

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println(rootCmd.Version)
    },
}

var infoCmd = &cobra.Command{
    Use:   "info",
    Short: "Print build information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("hCTF2 %s\n", rootCmd.Version)
        fmt.Printf("  go:      %s\n", runtime.Version())
        fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
        fmt.Printf("  cpus:    %d\n", runtime.NumCPU())
    },
}
```

Add in `init()`: `rootCmd.AddCommand(versionCmd, infoCmd)`

**Step 2: Run all tests**

```bash
go test ./...
```
Expected: all pass.

**Step 3: Build and smoke test the CLI**

```bash
go build -o hctf2 .
./hctf2 --help
./hctf2 serve --help
./hctf2 challenge --help
./hctf2 version
./hctf2 info
```

**Step 4: Run the server and test CLI against it**

```bash
./hctf2 serve --port 8092 --dev --db /tmp/cli-test.db \
  --admin-email admin@test.com --admin-password testpass123 &
./hctf2 login --server http://localhost:8092 --email admin@test.com --password testpass123
./hctf2 status
./hctf2 challenge list
./hctf2 user list
```

**Step 5: Commit and push**

```bash
git add cmd/root.go cmd/serve.go
git commit -m "feat(cli): wire version/info as subcommands, complete CLI"
git push origin HEAD
```

---

## Task 11: Update docs and CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` — update version, add CLI section
- Modify: `docs/plans/2026-03-10-cli-design.md` — mark status as Implemented

**Step 1: Update CLAUDE.md**

In the "Current version" line, bump to `v0.7.0`.

Add a section "## CLI Usage" with the command tree and examples.

**Step 2: Commit**

```bash
git add CLAUDE.md docs/plans/2026-03-10-cli-design.md
git commit -m "docs: document CLI usage, bump version to v0.7.0 in CLAUDE.md"
```

---

## Notes

- `newClient()` helper in `cmd/challenge.go` is shared by all cmd files — if it gets complex, extract to `cmd/common.go`
- The `jsonBody` helper in `client.go` needs a `bytes` import — add it in that task
- `cmd/serve.go` must call `getEnv()` — either copy the helper or move it to a shared `cmd/util.go`
- All charmbracelet styles degrade gracefully in non-color terminals (lipgloss detects NO_COLOR)
- After Task 5, run `go test ./handlers_test.go` to confirm the server handler tests still pass
