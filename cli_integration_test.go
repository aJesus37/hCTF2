package main

// CLI integration tests — exercise every command, subcommand, and flag.
//
// Strategy: TestMain builds the binary once, starts hctf2 serve as a
// subprocess on a free port, logs in as admin, and stores the server URL
// and config-file path in package-level variables.  Each test then calls
// runCLI() which execs the binary with HCTF2_CONFIG pointing at the temp
// config.  The suite is self-contained; no mock servers needed.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// ── package-level state set by TestMain ─────────────────────────────────────

var (
	cliBinary  string // path to built binary
	cliServer  string // e.g. "http://localhost:54321"
	cliConfig  string // path to HCTF2_CONFIG file pre-loaded with admin token
	serverProc *exec.Cmd
)

const (
	adminEmail = "admin@cli-test.local"
	adminPass  = "testpass123"
)

// ── TestMain ────────────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	code := func() int {
		// 1. Build binary into a temp dir so we don't clobber ./hctf2.
		tmpDir, err := os.MkdirTemp("", "hctf2-cli-test-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: MkdirTemp: %v\n", err)
			return 1
		}
		defer os.RemoveAll(tmpDir)

		cliBinary = filepath.Join(tmpDir, "hctf2-test")
		build := exec.Command("go", "build", "-o", cliBinary, ".")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: build failed: %v\n", err)
			return 1
		}

		// 2. Pick a free port and start the server.
		port, err := freePort()
		if err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: freePort: %v\n", err)
			return 1
		}
		cliServer = fmt.Sprintf("http://localhost:%d", port)

		dbPath := filepath.Join(tmpDir, "test.db")
		serverProc = exec.Command(cliBinary, "serve",
			"--port", fmt.Sprintf("%d", port),
			"--dev",
			"--db", dbPath,
			"--admin-email", adminEmail,
			"--admin-password", adminPass,
		)
		serverProc.Stdout = os.Stdout
		serverProc.Stderr = os.Stderr
		if err := serverProc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: server start: %v\n", err)
			return 1
		}
		defer func() {
			_ = serverProc.Process.Kill()
			_ = serverProc.Wait()
		}()

		// 3. Wait until healthz responds (up to 10 s).
		if !waitHealthy(cliServer, 10*time.Second) {
			fmt.Fprintf(os.Stderr, "TestMain: server never became healthy\n")
			return 1
		}

		// 4. Log in and write config file.
		cliConfig = filepath.Join(tmpDir, "config.yaml")
		stdout, stderr, code := runCLIRaw(
			"login",
			"--server", cliServer,
			"--email", adminEmail,
			"--password", adminPass,
		)
		if code != 0 {
			fmt.Fprintf(os.Stderr, "TestMain: login failed (code %d)\nstdout: %s\nstderr: %s\n",
				code, stdout, stderr)
			return 1
		}

		return m.Run()
	}()
	os.Exit(code)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// runCLI runs the CLI binary with the pre-configured HCTF2_CONFIG and returns
// stdout, stderr, and exit code.  It automatically injects --server so tests
// do not need to repeat it.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	return runCLIRaw(args...)
}

// runCLIRaw runs the binary without the test helper bookkeeping.
func runCLIRaw(args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command(cliBinary, args...)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+cliConfig)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return
}

// runCLIJSON runs a command and unmarshals stdout into v.
func runCLIJSON(t *testing.T, v any, args ...string) {
	t.Helper()
	stdout, stderr, code := runCLI(t, args...)
	if code != 0 {
		t.Fatalf("command failed (code %d)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("json.Unmarshal: %v\nstdout: %s", err, stdout)
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func waitHealthy(baseURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// assertSuccess fails the test if exitCode != 0.
func assertSuccess(t *testing.T, stdout, stderr string, exitCode int) {
	t.Helper()
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
	}
}

// assertError fails the test if exitCode == 0.
func assertError(t *testing.T, stdout, stderr string, exitCode int) {
	t.Helper()
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// assertContains fails if s does not contain substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

// assertNotContains fails if s contains substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected %q NOT to contain %q", s, substr)
	}
}

// ── version / info ───────────────────────────────────────────────────────────

func TestCLIVersion(t *testing.T) {
	stdout, stderr, code := runCLI(t, "version")
	assertSuccess(t, stdout, stderr, code)
	// version string must be non-empty
	if strings.TrimSpace(stdout) == "" {
		t.Error("expected non-empty version output")
	}
}

func TestCLIVersionFlag(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--version")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "hctf2")
}

func TestCLIInfo(t *testing.T) {
	stdout, stderr, code := runCLI(t, "info")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "hCTF2")
	assertContains(t, stdout, "go:")
	assertContains(t, stdout, "os/arch:")
	assertContains(t, stdout, "cpus:")
}

// ── help / no completion command ─────────────────────────────────────────────

func TestCLIHelpNoCompletion(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--help")
	assertSuccess(t, stdout, stderr, code)
	assertNotContains(t, stdout, "completion")
}

func TestCLIHelpListsAllCommands(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--help")
	assertSuccess(t, stdout, stderr, code)
	for _, cmd := range []string{"challenge", "competition", "flag", "login", "logout",
		"serve", "status", "team", "user", "version", "info"} {
		assertContains(t, stdout, cmd)
	}
}

// ── login / logout / status ──────────────────────────────────────────────────

func TestCLILoginSuccess(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", adminEmail,
		"--password", adminPass,
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("login failed: %v\nstderr: %s", err, errBuf.String())
	}
	assertContains(t, outBuf.String(), "Logged in")
}

func TestCLILoginMissingCredentials(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "login", "--server", cliServer)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when email/password missing")
	}
}

func TestCLILoginWrongPassword(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", adminEmail,
		"--password", "wrongpassword",
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error on wrong password")
	}
}

func TestCLILoginQuietFlag(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", adminEmail,
		"--password", adminPass,
		"--quiet",
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("login --quiet failed: %v", err)
	}
	if strings.TrimSpace(outBuf.String()) != "" {
		t.Errorf("--quiet should suppress output, got: %q", outBuf.String())
	}
}

func TestCLIStatus(t *testing.T) {
	stdout, stderr, code := runCLI(t, "status")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "Server:")
	assertContains(t, stdout, "Auth:")
	assertContains(t, stdout, adminEmail)
}

func TestCLIStatusJSON(t *testing.T) {
	var cfg map[string]any
	runCLIJSON(t, &cfg, "status", "--json")
	if _, ok := cfg["server"]; !ok {
		t.Error("JSON status missing 'server' field")
	}
	if _, ok := cfg["token"]; !ok {
		t.Error("JSON status missing 'token' field")
	}
}

func TestCLILogout(t *testing.T) {
	// Use a separate config so we don't break the shared admin session.
	tmpCfg := t.TempDir() + "/cfg.yaml"
	// First login to create a valid config.
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", adminEmail,
		"--password", adminPass,
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	if err := cmd.Run(); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	// Now logout.
	cmd = exec.Command(cliBinary, "logout")
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	assertContains(t, outBuf.String(), "Logged out")
}

func TestCLILogoutQuietFlag(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", adminEmail,
		"--password", adminPass,
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	_ = cmd.Run()

	cmd = exec.Command(cliBinary, "logout", "--quiet")
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("logout --quiet failed: %v", err)
	}
	if strings.TrimSpace(outBuf.String()) != "" {
		t.Errorf("--quiet should suppress output, got %q", outBuf.String())
	}
}

// ── not-logged-in error ───────────────────────────────────────────────────────

func TestCLIErrorNotLoggedIn(t *testing.T) {
	tmpCfg := t.TempDir() + "/cfg.yaml"
	cmd := exec.Command(cliBinary, "challenge", "list", "--server", cliServer)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

// ── challenge list ────────────────────────────────────────────────────────────

func TestCLIChallengeList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "challenge", "list")
	assertSuccess(t, stdout, stderr, code)
	// Table headers should be present
	assertContains(t, stdout, "TITLE")
	assertContains(t, stdout, "CATEGORY")
	assertContains(t, stdout, "PTS")
}

func TestCLIChallengeListAlias(t *testing.T) {
	stdout, stderr, code := runCLI(t, "ch", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "TITLE")
}

func TestCLIChallengeListJSON(t *testing.T) {
	var challenges []map[string]any
	runCLIJSON(t, &challenges, "challenge", "list", "--json")
	// May be empty initially — just validate we get a JSON array.
	if challenges == nil {
		challenges = []map[string]any{}
	}
}

// ── challenge create ──────────────────────────────────────────────────────────

func TestCLIChallengeCreate(t *testing.T) {
	stdout, stderr, code := runCLI(t,
		"challenge", "create",
		"--title", "Test Challenge Alpha",
		"--category", "web",
		"--difficulty", "easy",
		"--points", "150",
	)
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "Test Challenge Alpha")
}

func TestCLIChallengeCreateQuiet(t *testing.T) {
	stdout, stderr, code := runCLI(t,
		"challenge", "create",
		"--title", "Quiet Challenge",
		"--category", "crypto",
		"--difficulty", "medium",
		"--points", "200",
		"--quiet",
	)
	assertSuccess(t, stdout, stderr, code)
	// --quiet should print only the ID (a UUID-like string)
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("--quiet should output the challenge ID")
	}
	// Should NOT contain the "Created challenge" prose
	assertNotContains(t, stdout, "Created challenge")
}

func TestCLIChallengeCreateWithDescription(t *testing.T) {
	stdout, stderr, code := runCLI(t,
		"challenge", "create",
		"--title", "Described Challenge",
		"--category", "pwn",
		"--difficulty", "hard",
		"--points", "500",
		"--description", "A really hard challenge",
	)
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "Described Challenge")
}

// ── challenge get ─────────────────────────────────────────────────────────────

func TestCLIChallengeGet(t *testing.T) {
	// Create a challenge and retrieve its ID via --quiet.
	_, _, _ = runCLI(t, // ensure at least one exists
		"challenge", "create",
		"--title", "GetMe Challenge",
		"--category", "forensics",
		"--difficulty", "easy",
		"--points", "100",
		"--quiet",
	)

	// Get list to find an ID.
	var challenges []map[string]any
	runCLIJSON(t, &challenges, "challenge", "list", "--json")
	if len(challenges) == 0 {
		t.Skip("no challenges to get")
	}
	id := challenges[0]["id"].(string)

	stdout, stderr, code := runCLI(t, "challenge", "get", id)
	assertSuccess(t, stdout, stderr, code)
	// Must show [category / difficulty]  N pts — not [ / ] 0 pts.
	assertContains(t, stdout, "pts")
	assertNotContains(t, stdout, "[ / ]")
}

func TestCLIChallengeGetJSON(t *testing.T) {
	var challenges []map[string]any
	runCLIJSON(t, &challenges, "challenge", "list", "--json")
	if len(challenges) == 0 {
		t.Skip("no challenges to get")
	}
	id := challenges[0]["id"].(string)

	var ch map[string]any
	runCLIJSON(t, &ch, "challenge", "get", id, "--json")
	if ch["id"] == nil {
		t.Error("JSON challenge get missing 'id' field")
	}
	if ch["name"] == nil {
		t.Error("JSON challenge get missing 'name' field")
	}
}

func TestCLIChallengeGetMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "challenge", "get")
	assertError(t, "", stderr, code)
}

func TestCLIChallengeGetNotFound(t *testing.T) {
	_, _, code := runCLI(t, "challenge", "get", "nonexistent-id-xxx")
	assertError(t, "", "", code)
}

// ── challenge delete ──────────────────────────────────────────────────────────

func TestCLIChallengeDelete(t *testing.T) {
	// Create then delete.
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "ToDelete",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create failed (code %d): %s", code, stdout)
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "challenge", "delete", id)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, id)
}

func TestCLIChallengeDeleteQuiet(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "ToDeleteQuiet",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "challenge", "delete", id, "--quiet")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) != "" {
		t.Errorf("--quiet should suppress delete output, got %q", out)
	}
}

func TestCLIChallengeDeleteMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "challenge", "delete")
	assertError(t, "", stderr, code)
}

// ── competition list ──────────────────────────────────────────────────────────

func TestCLICompetitionList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "competition", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "NAME")
	assertContains(t, stdout, "STATUS")
}

func TestCLICompetitionListAlias(t *testing.T) {
	stdout, stderr, code := runCLI(t, "comp", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "NAME")
}

func TestCLICompetitionListJSON(t *testing.T) {
	var comps []map[string]any
	runCLIJSON(t, &comps, "competition", "list", "--json")
	if comps == nil {
		comps = []map[string]any{}
	}
}

// ── competition create ────────────────────────────────────────────────────────

func TestCLICompetitionCreate(t *testing.T) {
	stdout, stderr, code := runCLI(t, "competition", "create", "My Test CTF")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "My Test CTF")
}

func TestCLICompetitionCreateQuiet(t *testing.T) {
	stdout, stderr, code := runCLI(t, "competition", "create", "Quiet CTF", "--quiet")
	assertSuccess(t, stdout, stderr, code)
	// --quiet emits only the ID (integer)
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("--quiet should print competition ID")
	}
	assertNotContains(t, stdout, "Created competition")
}

func TestCLICompetitionCreateMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "competition", "create")
	assertError(t, "", stderr, code)
}

// ── competition start / end ───────────────────────────────────────────────────

func TestCLICompetitionStartEnd(t *testing.T) {
	// Create a competition to start/end.
	stdout, _, code := runCLI(t, "competition", "create", "StartEnd CTF", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "competition", "start", id)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, id)

	out, stderr, code = runCLI(t, "competition", "end", id)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, id)
}

func TestCLICompetitionStartQuiet(t *testing.T) {
	stdout, _, _ := runCLI(t, "competition", "create", "StartQuiet CTF", "--quiet")
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "competition", "start", id, "--quiet")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) != "" {
		t.Errorf("--quiet should suppress start output, got %q", out)
	}
}

func TestCLICompetitionEndQuiet(t *testing.T) {
	stdout, _, _ := runCLI(t, "competition", "create", "EndQuiet CTF", "--quiet")
	id := strings.TrimSpace(stdout)
	runCLI(t, "competition", "start", id, "--quiet")

	out, stderr, code := runCLI(t, "competition", "end", id, "--quiet")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) != "" {
		t.Errorf("--quiet should suppress end output, got %q", out)
	}
}

func TestCLICompetitionStartInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "competition", "start", "not-a-number")
	assertError(t, "", "", code)
}

func TestCLICompetitionEndInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "competition", "end", "not-a-number")
	assertError(t, "", "", code)
}

func TestCLICompetitionStartMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "competition", "start")
	assertError(t, "", stderr, code)
}

// ── team list ─────────────────────────────────────────────────────────────────

func TestCLITeamList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "team", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "NAME")
}

func TestCLITeamListJSON(t *testing.T) {
	var teams []map[string]any
	runCLIJSON(t, &teams, "team", "list", "--json")
	if teams == nil {
		teams = []map[string]any{}
	}
}

// ── team create ───────────────────────────────────────────────────────────────

func TestCLITeamCreate(t *testing.T) {
	stdout, stderr, code := runCLI(t, "team", "create", "Alpha Squad")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "Alpha Squad")
	// Leave the team so subsequent team tests can create new teams.
	leaveTeam(t)
}

func TestCLITeamCreateQuiet(t *testing.T) {
	stdout, stderr, code := runCLI(t, "team", "create", "QuietTeam", "--quiet")
	assertSuccess(t, stdout, stderr, code)
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("--quiet should print team ID")
	}
	assertNotContains(t, stdout, "Created team")
	leaveTeam(t)
}

func TestCLITeamCreateMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "team", "create")
	assertError(t, "", stderr, code)
}

// ── team get ──────────────────────────────────────────────────────────────────

func TestCLITeamGet(t *testing.T) {
	stdout, _, code := runCLI(t, "team", "create", "GetTeam", "--quiet")
	if code != 0 {
		t.Fatalf("create team failed")
	}
	id := strings.TrimSpace(stdout)
	defer leaveTeam(t)

	out, stderr, code := runCLI(t, "team", "get", id)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "GetTeam")
}

func TestCLITeamGetJSON(t *testing.T) {
	stdout, _, code := runCLI(t, "team", "create", "GetTeamJSON", "--quiet")
	if code != 0 {
		t.Fatalf("create team failed")
	}
	id := strings.TrimSpace(stdout)
	defer leaveTeam(t)

	var envelope map[string]any
	runCLIJSON(t, &envelope, "team", "get", id, "--json")
	teamObj, ok := envelope["team"].(map[string]any)
	if !ok || teamObj["id"] == nil {
		t.Error("JSON team get missing 'team.id' field")
	}
}

func TestCLITeamGetMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "team", "get")
	assertError(t, "", stderr, code)
}

// ── team join ─────────────────────────────────────────────────────────────────

func TestCLITeamJoinInvalidCode(t *testing.T) {
	// Joining with a bogus invite code must return an error.
	_, _, code := runCLI(t, "team", "join", "bogus-invite-code-xxx")
	assertError(t, "", "", code)
}

func TestCLITeamJoinMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "team", "join")
	assertError(t, "", stderr, code)
}

// ── flag submit ───────────────────────────────────────────────────────────────

func TestCLIFlagSubmitMissingArgs(t *testing.T) {
	_, stderr, code := runCLI(t, "flag", "submit")
	assertError(t, "", stderr, code)
}

func TestCLIFlagSubmitWrongFlag(t *testing.T) {
	// Submit to a non-existent question; should fail.
	_, _, code := runCLI(t, "flag", "submit", "nonexistent-question-id", "flag{wrong}")
	assertError(t, "", "", code)
}

// ── user list ─────────────────────────────────────────────────────────────────

func TestCLIUserList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "user", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "EMAIL")
	assertContains(t, stdout, adminEmail)
}

func TestCLIUserListJSON(t *testing.T) {
	var users []map[string]any
	runCLIJSON(t, &users, "user", "list", "--json")
	if len(users) == 0 {
		t.Fatal("expected at least one user (admin)")
	}
	found := false
	for _, u := range users {
		if u["email"] == adminEmail {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("admin user %s not found in user list JSON", adminEmail)
	}
}

// ── user promote / demote ─────────────────────────────────────────────────────

func TestCLIUserPromoteDemote(t *testing.T) {
	// Register a new user via the API so we have someone to promote/demote.
	userID := createTestUser(t, "promote-test@example.com", "promotepass123")

	out, stderr, code := runCLI(t, "user", "promote", userID)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, userID)
	assertContains(t, out, "promoted")

	out, stderr, code = runCLI(t, "user", "demote", userID)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, userID)
	assertContains(t, out, "demoted")
}

func TestCLIUserPromoteQuiet(t *testing.T) {
	userID := createTestUser(t, "promote-quiet@example.com", "promotepass123")

	out, stderr, code := runCLI(t, "user", "promote", userID, "--quiet")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) != "" {
		t.Errorf("--quiet should suppress output, got %q", out)
	}
}

func TestCLIUserPromoteMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "user", "promote")
	assertError(t, "", stderr, code)
}

// ── user delete ───────────────────────────────────────────────────────────────

func TestCLIUserDelete(t *testing.T) {
	userID := createTestUser(t, "delete-me@example.com", "deletepass123")

	out, stderr, code := runCLI(t, "user", "delete", userID)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, userID)
}

func TestCLIUserDeleteQuiet(t *testing.T) {
	userID := createTestUser(t, "delete-quiet@example.com", "deletepass123")

	out, stderr, code := runCLI(t, "user", "delete", userID, "--quiet")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) != "" {
		t.Errorf("--quiet should suppress output, got %q", out)
	}
}

func TestCLIUserDeleteMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "user", "delete")
	assertError(t, "", stderr, code)
}

// ── --server flag override ────────────────────────────────────────────────────

func TestCLIServerFlagOverride(t *testing.T) {
	// --server pointing at a non-existent address must fail (connection refused).
	_, _, code := runCLI(t, "challenge", "list", "--server", "http://localhost:1")
	assertError(t, "", "", code)
}

// ── challenge browse error in non-TTY ─────────────────────────────────────────

func TestCLIChallengeBrowseRequiresTTY(t *testing.T) {
	// When stdin is not a TTY (always in tests), browse should error.
	_, _, code := runCLI(t, "challenge", "browse")
	assertError(t, "", "", code)
}

// ── createTestUser helper ──────────────────────────────────────────────────────

// disbandTeam calls the disband-team API endpoint for the admin session so
// subsequent team-create tests start from a clean state.  The admin is always
// the owner of any team they create, so /leave returns 403; disband is the
// correct endpoint for owners.
func leaveTeam(t *testing.T) {
	t.Helper()
	token := adminToken(t)
	if token == "" {
		return
	}
	req, _ := http.NewRequest("POST", cliServer+"/api/teams/disband", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// adminToken reads the JWT from the shared admin config file.
func adminToken(t *testing.T) string {
	t.Helper()
	cfg, err := os.ReadFile(cliConfig)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(cfg), "\n") {
		if strings.HasPrefix(line, "token:") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "token:"))
			return strings.Trim(token, `"'`)
		}
	}
	return ""
}

// ── register ─────────────────────────────────────────────────────────────────

func TestCLIRegisterSuccess(t *testing.T) {
	stdout, stderr, code := runCLI(t, "register",
		"--server", cliServer,
		"--email", "unique1@test.local",
		"--name", "TestUser",
		"--password", "pass123",
	)
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "unique1@test.local")
}

func TestCLIRegisterDuplicateEmail(t *testing.T) {
	// First registration should succeed.
	stdout, stderr, code := runCLI(t, "register",
		"--server", cliServer,
		"--email", "dup-reg@test.local",
		"--name", "DupUser",
		"--password", "pass123",
	)
	assertSuccess(t, stdout, stderr, code)

	// Second registration with same email must fail.
	_, _, code2 := runCLI(t, "register",
		"--server", cliServer,
		"--email", "dup-reg@test.local",
		"--name", "DupUser2",
		"--password", "pass123",
	)
	assertError(t, "", "", code2)
}

func TestCLIRegisterMissingFlags(t *testing.T) {
	// In non-TTY mode, missing email/password must error.
	_, _, code := runCLI(t, "register", "--server", cliServer)
	assertError(t, "", "", code)
}

// ── scoreboard ────────────────────────────────────────────────────────────────

func TestCLIScoreboard(t *testing.T) {
	stdout, stderr, code := runCLI(t, "scoreboard")
	assertSuccess(t, stdout, stderr, code)
	// Either shows the table header or the empty message.
	if !strings.Contains(stdout, "RANK") && !strings.Contains(stdout, "No scoreboard") {
		t.Errorf("expected scoreboard output, got: %q", stdout)
	}
}

func TestCLIScoreboardJSON(t *testing.T) {
	var entries []map[string]any
	runCLIJSON(t, &entries, "scoreboard", "--json")
	if entries == nil {
		entries = []map[string]any{}
	}
}

// ── challenge update ──────────────────────────────────────────────────────────

func TestCLIChallengeUpdate(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "Original Title",
		"--category", "web",
		"--difficulty", "easy",
		"--points", "100",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create failed (code %d): %s", code, stdout)
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t,
		"challenge", "update", id,
		"--title", "Updated Title",
		"--category", "web",
		"--difficulty", "easy",
		"--points", "200",
	)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "Updated")
}

func TestCLIChallengeUpdateQuiet(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "UpdateQuiet",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "100",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t,
		"challenge", "update", id,
		"--title", "UpdatedQuiet",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "150",
		"--quiet",
	)
	assertSuccess(t, out, stderr, code)
	// --quiet emits only the challenge ID, not prose like "Updated challenge"
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		t.Error("--quiet should output the challenge ID")
	}
	assertNotContains(t, out, "Updated challenge")
}

func TestCLIChallengeUpdateMissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "challenge", "update")
	assertError(t, "", stderr, code)
}

// ── question list / create / delete ──────────────────────────────────────────

func TestCLIQuestionList(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "QListChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "question", "list", chID)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "NAME")
}

func TestCLIQuestionCreate(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "QCreateChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t,
		"question", "create",
		"--challenge", chID,
		"--name", "Q1",
		"--flag", "flag{test}",
		"--points", "100",
		"--quiet",
	)
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) == "" {
		t.Error("--quiet should output the question ID")
	}
}

func TestCLIQuestionDelete(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "QDeleteChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	stdout, _, code = runCLI(t,
		"question", "create",
		"--challenge", chID,
		"--name", "QToDelete",
		"--flag", "flag{del}",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create question failed")
	}
	qID := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "question", "delete", qID)
	assertSuccess(t, out, stderr, code)
}

func TestCLIQuestionCreateMissingFlag(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "QMissingFlagChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	// Missing --flag must error in non-TTY mode.
	_, _, code = runCLI(t,
		"question", "create",
		"--challenge", chID,
		"--name", "Q1",
		"--points", "100",
	)
	assertError(t, "", "", code)
}

func TestCLIQuestionListJSON(t *testing.T) {
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "QListJSONChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	var qs []map[string]any
	runCLIJSON(t, &qs, "question", "list", chID, "--json")
	if qs == nil {
		qs = []map[string]any{}
	}
}

// ── hint list / create / delete ───────────────────────────────────────────────

// hintTestSetup creates a challenge and question for hint tests, returns question ID.
func hintTestSetup(t *testing.T) string {
	t.Helper()
	stdout, _, code := runCLI(t,
		"challenge", "create",
		"--title", "HintChallenge-"+t.Name(),
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(stdout)

	stdout, _, code = runCLI(t,
		"question", "create",
		"--challenge", chID,
		"--name", "HintQ",
		"--flag", "flag{hint}",
		"--points", "50",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create question failed")
	}
	return strings.TrimSpace(stdout)
}

func TestCLIHintList(t *testing.T) {
	qID := hintTestSetup(t)
	out, stderr, code := runCLI(t, "hint", "list", qID)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "COST")
}

func TestCLIHintCreate(t *testing.T) {
	qID := hintTestSetup(t)
	out, stderr, code := runCLI(t,
		"hint", "create",
		"--question", qID,
		"--content", "Try harder",
		"--cost", "10",
		"--quiet",
	)
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) == "" {
		t.Error("--quiet should output hint ID")
	}
}

func TestCLIHintDelete(t *testing.T) {
	qID := hintTestSetup(t)

	stdout, _, code := runCLI(t,
		"hint", "create",
		"--question", qID,
		"--content", "Delete me hint",
		"--cost", "5",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create hint failed")
	}
	hID := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "hint", "delete", hID)
	assertSuccess(t, out, stderr, code)
}

func TestCLIHintListJSON(t *testing.T) {
	qID := hintTestSetup(t)
	var hints []map[string]any
	runCLIJSON(t, &hints, "hint", "list", qID, "--json")
	if hints == nil {
		hints = []map[string]any{}
	}
}

// ── category list / create / delete ──────────────────────────────────────────

func TestCLICategoryList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "category", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "NAME")
}

func TestCLICategoryCreate(t *testing.T) {
	out, stderr, code := runCLI(t, "category", "create", "--name", "TestCat")
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "TestCat")
}

func TestCLICategoryDelete(t *testing.T) {
	// Create a category, find its ID via JSON list, then delete.
	_, _, code := runCLI(t, "category", "create", "--name", "CatToDelete")
	if code != 0 {
		t.Fatalf("create category failed")
	}

	var cats []map[string]any
	runCLIJSON(t, &cats, "category", "list", "--json")
	var id string
	for _, c := range cats {
		if c["name"] == "CatToDelete" {
			id = c["id"].(string)
			break
		}
	}
	if id == "" {
		t.Fatal("created category not found in list")
	}

	out, stderr, code := runCLI(t, "category", "delete", id)
	assertSuccess(t, out, stderr, code)
}

func TestCLICategoryListJSON(t *testing.T) {
	var cats []map[string]any
	runCLIJSON(t, &cats, "category", "list", "--json")
	if cats == nil {
		cats = []map[string]any{}
	}
}

// ── difficulty list / create / delete ─────────────────────────────────────────

func TestCLIDifficultyList(t *testing.T) {
	stdout, stderr, code := runCLI(t, "difficulty", "list")
	assertSuccess(t, stdout, stderr, code)
	assertContains(t, stdout, "NAME")
}

func TestCLIDifficultyCreate(t *testing.T) {
	out, stderr, code := runCLI(t, "difficulty", "create", "--name", "TestDiff")
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "TestDiff")
}

func TestCLIDifficultyDelete(t *testing.T) {
	_, _, code := runCLI(t, "difficulty", "create", "--name", "DiffToDelete")
	if code != 0 {
		t.Fatalf("create difficulty failed")
	}

	var diffs []map[string]any
	runCLIJSON(t, &diffs, "difficulty", "list", "--json")
	var id string
	for _, d := range diffs {
		if d["name"] == "DiffToDelete" {
			id = d["id"].(string)
			break
		}
	}
	if id == "" {
		t.Fatal("created difficulty not found in list")
	}

	out, stderr, code := runCLI(t, "difficulty", "delete", id)
	assertSuccess(t, out, stderr, code)
}

func TestCLIDifficultyListJSON(t *testing.T) {
	var diffs []map[string]any
	runCLIJSON(t, &diffs, "difficulty", "list", "--json")
	if diffs == nil {
		diffs = []map[string]any{}
	}
}

// ── team leave and disband ───────────────────────────────────────────────────

func TestCLITeamLeave(t *testing.T) {
	// Create a second user and a temp config for them.
	secondEmail := "team-leave-user@test.local"
	secondPass := "leavepass123"
	createTestUser(t, secondEmail, secondPass)

	tmpCfg := t.TempDir() + "/cfg.yaml"
	// Login as second user.
	cmd := exec.Command(cliBinary, "login",
		"--server", cliServer,
		"--email", secondEmail,
		"--password", secondPass,
	)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	if err := cmd.Run(); err != nil {
		t.Fatalf("login second user failed: %v", err)
	}

	// Second user creates a team.
	cmd = exec.Command(cliBinary, "team", "create", "LeaveTeam", "--quiet")
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	if err := cmd.Run(); err != nil {
		t.Fatalf("second user create team failed: %v", err)
	}

	// Second user leaves the team (owner can also leave/disband via leave endpoint).
	// Actually owners can't leave; disband instead. But team leave for a non-owner works.
	// Let's have admin join the team by having second user regen invite, admin joins, then admin leaves.
	var outBuf strings.Builder
	cmd = exec.Command(cliBinary, "team", "invite-regen")
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("regen invite failed: %v", err)
	}
	inviteCode := strings.TrimSpace(outBuf.String())

	// Admin joins team.
	out, stderr, code := runCLI(t, "team", "join", inviteCode)
	assertSuccess(t, out, stderr, code)

	// Admin leaves team.
	out, stderr, code = runCLI(t, "team", "leave")
	assertSuccess(t, out, stderr, code)

	// Cleanup: second user disbands team.
	cmd = exec.Command(cliBinary, "team", "disband")
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg)
	_ = cmd.Run()
}

func TestCLITeamDisband(t *testing.T) {
	// Admin creates a team, then disbands it.
	stdout, _, code := runCLI(t, "team", "create", "DisbandTeam", "--quiet")
	if code != 0 {
		t.Fatalf("create team failed")
	}
	_ = strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "team", "disband")
	assertSuccess(t, out, stderr, code)
	// No need to call leaveTeam since team was disbanded.
}

// ── team invite-regen ─────────────────────────────────────────────────────────

func TestCLITeamInviteRegen(t *testing.T) {
	stdout, _, code := runCLI(t, "team", "create", "InviteRegenTeam", "--quiet")
	if code != 0 {
		t.Fatalf("create team failed")
	}
	_ = strings.TrimSpace(stdout)
	defer leaveTeam(t)

	out, stderr, code := runCLI(t, "team", "invite-regen")
	assertSuccess(t, out, stderr, code)
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty invite code output")
	}
}

// ── competition get / delete / add-challenge / remove-challenge / freeze / unfreeze / register ──

func TestCLICompetitionGet(t *testing.T) {
	stdout, _, code := runCLI(t, "competition", "create", "GetComp", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "competition", "get", id)
	assertSuccess(t, out, stderr, code)
	assertContains(t, out, "GetComp")
}

func TestCLICompetitionGetJSON(t *testing.T) {
	stdout, _, code := runCLI(t, "competition", "create", "GetCompJSON", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	id := strings.TrimSpace(stdout)

	var co map[string]any
	runCLIJSON(t, &co, "competition", "get", id, "--json")
	if co["id"] == nil {
		t.Error("JSON competition get missing 'id' field")
	}
}

func TestCLICompetitionDelete(t *testing.T) {
	stdout, _, code := runCLI(t, "competition", "create", "DeleteComp", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "competition", "delete", id)
	assertSuccess(t, out, stderr, code)
}

func TestCLICompetitionAddRemoveChallenge(t *testing.T) {
	compOut, _, code := runCLI(t, "competition", "create", "AddRemoveComp", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	compID := strings.TrimSpace(compOut)

	chOut, _, code := runCLI(t,
		"challenge", "create",
		"--title", "CompChallenge",
		"--category", "misc",
		"--difficulty", "easy",
		"--points", "100",
		"--quiet",
	)
	if code != 0 {
		t.Fatalf("create challenge failed")
	}
	chID := strings.TrimSpace(chOut)

	out, stderr, code := runCLI(t, "competition", "add-challenge", compID, chID)
	assertSuccess(t, out, stderr, code)

	out, stderr, code = runCLI(t, "competition", "remove-challenge", compID, chID)
	assertSuccess(t, out, stderr, code)
}

func TestCLICompetitionFreezeUnfreeze(t *testing.T) {
	stdout, _, code := runCLI(t, "competition", "create", "FreezeComp", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	id := strings.TrimSpace(stdout)

	out, stderr, code := runCLI(t, "competition", "freeze", id)
	assertSuccess(t, out, stderr, code)

	out, stderr, code = runCLI(t, "competition", "unfreeze", id)
	assertSuccess(t, out, stderr, code)
}

func TestCLICompetitionRegister(t *testing.T) {
	// Create and start a competition.
	stdout, _, code := runCLI(t, "competition", "create", "RegisterComp", "--quiet")
	if code != 0 {
		t.Fatalf("create competition failed")
	}
	compID := strings.TrimSpace(stdout)

	_, _, _ = runCLI(t, "competition", "start", compID)

	// Create a team for admin.
	_, _, _ = runCLI(t, "team", "create", "RegisterTeam", "--quiet")
	defer leaveTeam(t)

	out, stderr, code := runCLI(t, "competition", "register", compID)
	assertSuccess(t, out, stderr, code)
}

// createTestUser registers a user via the API and returns their ID.
func createTestUser(t *testing.T, email, password string) string {
	t.Helper()
	resp, err := http.PostForm(cliServer+"/api/auth/register", map[string][]string{
		"email":    {email},
		"password": {password},
		"name":     {"Test User"},
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("register user returned %d", resp.StatusCode)
	}

	// Fetch user list and find the ID.
	var users []map[string]any
	runCLIJSON(t, &users, "user", "list", "--json")
	for _, u := range users {
		if u["email"] == email {
			return u["id"].(string)
		}
	}
	t.Fatalf("newly registered user %s not found in user list", email)
	return ""
}

// createTestChallenge creates a challenge via CLI and returns its ID.
func createTestChallenge(t *testing.T, title string) string {
	t.Helper()
	stdout, stderr, code := runCLI(t, "challenge", "create",
		"--title", title, "--category", "web", "--difficulty", "easy",
		"--points", "100", "--description", "test", "--quiet")
	if code != 0 {
		t.Fatalf("createTestChallenge: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	return strings.TrimSpace(stdout)
}

type apiChallengeDetail struct {
	ID             string `json:"id"`
	Visible        bool   `json:"visible"`
	MinimumPoints  int    `json:"minimum_points"`
	DecayThreshold int    `json:"decay_threshold"`
	InitialPoints  int    `json:"initial_points"`
}

func apiGetChallenge(t *testing.T, id string) apiChallengeDetail {
	t.Helper()
	resp, err := http.Get(cliServer + "/api/challenges/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Challenge apiChallengeDetail `json:"challenge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return env.Challenge
}

func createTestQuestion(t *testing.T, challengeID, name, flag string, points int) string {
	t.Helper()
	out, stderr, code := runCLI(t, "question", "create",
		"--challenge", challengeID,
		"--name", name,
		"--flag", flag,
		"--points", strconv.Itoa(points),
		"--quiet")
	if code != 0 {
		t.Fatalf("createTestQuestion: code=%d stderr=%s", code, stderr)
	}
	return strings.TrimSpace(out)
}

type apiQuestion struct {
	Name string `json:"name"`
}

func apiGetQuestion(t *testing.T, challengeID string) apiQuestion {
	t.Helper()
	resp, err := http.Get(cliServer + "/api/challenges/" + challengeID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Questions []apiQuestion `json:"questions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Questions) == 0 {
		t.Fatal("no questions found")
	}
	return env.Questions[0]
}

func createTestHint(t *testing.T, questionID, content string, cost int) string {
	t.Helper()
	out, stderr, code := runCLI(t, "hint", "create",
		"--question", questionID,
		"--content", content,
		"--cost", strconv.Itoa(cost),
		"--quiet")
	if code != 0 {
		t.Fatalf("createTestHint: code=%d stderr=%s", code, stderr)
	}
	return strings.TrimSpace(out)
}

// ── Task 1: challenge update — visible, min-points, decay flags ───────────────

func TestCLIChallengeUpdateVisible(t *testing.T) {
	id := createTestChallenge(t, "VisTest")
	out, stderr, code := runCLI(t, "challenge", "update", id,
		"--title", "VisTest", "--category", "web",
		"--difficulty", "easy", "--points", "100", "--visible")
	assertSuccess(t, out, stderr, code)
	ch := apiGetChallenge(t, id)
	if !ch.Visible {
		t.Fatal("expected visible=true after update")
	}
}

func TestCLIChallengeUpdateDynamicScoring(t *testing.T) {
	id := createTestChallenge(t, "DynTest")
	out, stderr, code := runCLI(t, "challenge", "update", id,
		"--title", "DynTest", "--category", "web",
		"--difficulty", "easy", "--points", "500", "--min-points", "50", "--decay", "10")
	assertSuccess(t, out, stderr, code)
	ch := apiGetChallenge(t, id)
	if ch.MinimumPoints != 50 {
		t.Fatalf("expected min_points=50, got %d", ch.MinimumPoints)
	}
	if ch.DecayThreshold != 10 {
		t.Fatalf("expected decay=10, got %d", ch.DecayThreshold)
	}
}

// ── Task 2: challenge export / import ────────────────────────────────────────

func TestCLIChallengeExport(t *testing.T) {
	createTestChallenge(t, "ExportMe")
	out, stderr, code := runCLI(t, "challenge", "export")
	assertSuccess(t, out, stderr, code)
	var bundle struct {
		Challenges []json.RawMessage `json:"challenges"`
	}
	if err := json.Unmarshal([]byte(out), &bundle); err != nil {
		t.Fatalf("export output is not valid JSON bundle: %v\ngot: %s", err, out)
	}
	if len(bundle.Challenges) == 0 {
		t.Fatal("expected at least one challenge in export")
	}
}

func TestCLIChallengeExportToFile(t *testing.T) {
	createTestChallenge(t, "ExportFile")
	f := t.TempDir() + "/export.json"
	out, stderr, code := runCLI(t, "challenge", "export", "--output", f)
	assertSuccess(t, out, stderr, code)
	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	var bundle struct {
		Challenges []json.RawMessage `json:"challenges"`
	}
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("file content is not valid JSON bundle: %v", err)
	}
}

func TestCLIChallengeImport(t *testing.T) {
	exported, stderr, code := runCLI(t, "challenge", "export")
	assertSuccess(t, exported, stderr, code)
	f := t.TempDir() + "/import.json"
	if err := os.WriteFile(f, []byte(exported), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	out, stderr2, code2 := runCLI(t, "challenge", "import", f)
	assertSuccess(t, out, stderr2, code2)
	if !strings.Contains(out, "Imported") {
		t.Fatalf("expected 'Imported successfully', got: %s", out)
	}
}

func TestCLIChallengeExportJSON(t *testing.T) {
	// challenge export always outputs the bundle JSON object (--json is a no-op here)
	out, stderr, code := runCLI(t, "challenge", "export", "--json")
	assertSuccess(t, out, stderr, code)
	var bundle map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &bundle); err != nil {
		t.Fatalf("export output not valid JSON object: %v\ngot: %s", err, out)
	}
	if _, ok := bundle["challenges"]; !ok {
		t.Fatal("export bundle missing 'challenges' key")
	}
}

// ── Task 3: question update ───────────────────────────────────────────────────

func TestCLIQuestionUpdate(t *testing.T) {
	chID := createTestChallenge(t, "QUpdateCh")
	qID := createTestQuestion(t, chID, "OldName", "flag{old}", 100)
	out, stderr, code := runCLI(t, "question", "update", qID,
		"--name", "NewName", "--flag", "flag{new}", "--points", "200")
	assertSuccess(t, out, stderr, code)
	if !strings.Contains(out, "Updated") {
		t.Fatalf("expected 'Updated', got: %s", out)
	}
	q := apiGetQuestion(t, chID)
	if q.Name != "NewName" {
		t.Fatalf("expected name=NewName, got %s", q.Name)
	}
}

func TestCLIQuestionUpdateMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "question", "update")
	if code == 0 {
		t.Fatal("expected error for missing arg")
	}
}

// ── Task 4: hint update ───────────────────────────────────────────────────────

func TestCLIHintUpdate(t *testing.T) {
	chID := createTestChallenge(t, "HintUpdateCh")
	qID := createTestQuestion(t, chID, "HintQ", "flag{h}", 100)
	hID := createTestHint(t, qID, "old hint", 10)
	out, stderr, code := runCLI(t, "hint", "update", hID,
		"--content", "new hint text", "--cost", "20", "--order", "1")
	assertSuccess(t, out, stderr, code)
	if !strings.Contains(out, "Updated") {
		t.Fatalf("expected 'Updated', got: %s", out)
	}
}

func TestCLIHintUpdateMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "hint", "update")
	if code == 0 {
		t.Fatal("expected error for missing arg")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func createTestCompetition(t *testing.T, name string) string {
	t.Helper()
	out, stderr, code := runCLI(t, "competition", "create", name, "--quiet")
	if code != 0 {
		t.Fatalf("createTestCompetition: code=%d stderr=%s", code, stderr)
	}
	return strings.TrimSpace(out)
}

func runCLIWithErrorAndNoAuth(t *testing.T, args ...string) (string, error) {
	t.Helper()
	tmpCfg, err := os.CreateTemp("", "hctf2-noauth-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpCfg.Name())
	fmt.Fprintf(tmpCfg, "server: %s\n", cliServer)
	tmpCfg.Close()

	cmd := exec.Command(cliBinary, args...)
	cmd.Env = append(os.Environ(), "HCTF2_CONFIG="+tmpCfg.Name())
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	rerr := cmd.Run()
	combined := outBuf.String() + errBuf.String()
	if rerr != nil {
		return combined, rerr
	}
	return combined, nil
}

// ── Task 5: competition update / teams / blackout / scoreboard ────────────────

func TestCLICompetitionUpdate(t *testing.T) {
	compID := createTestCompetition(t, "UpdateComp")
	out, stderr, code := runCLI(t, "competition", "update", compID,
		"--name", "UpdatedComp", "--description", "new desc")
	assertSuccess(t, out, stderr, code)
	if !strings.Contains(out, "Updated") {
		t.Fatalf("expected 'Updated', got: %s", out)
	}
}

func TestCLICompetitionUpdateMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "competition", "update")
	if code == 0 {
		t.Fatal("expected error")
	}
}

func TestCLICompetitionTeams(t *testing.T) {
	compID := createTestCompetition(t, "TeamsComp")
	_, stderr, code := runCLI(t, "competition", "teams", compID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
}

func TestCLICompetitionTeamsJSON(t *testing.T) {
	compID := createTestCompetition(t, "TeamsJSONComp")
	out, stderr, code := runCLI(t, "competition", "teams", "--json", compID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("expected JSON array, got: %s err: %v", out, err)
	}
}

func TestCLICompetitionBlackout(t *testing.T) {
	compID := createTestCompetition(t, "BlackoutComp")
	out, stderr, code := runCLI(t, "competition", "blackout", compID)
	if code != 0 {
		t.Fatalf("blackout failed code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(out), "blackout") {
		t.Fatalf("expected blackout confirmation, got: %s", out)
	}
	out2, stderr2, code2 := runCLI(t, "competition", "unblackout", compID)
	if code2 != 0 {
		t.Fatalf("unblackout failed code=%d stderr=%s", code2, stderr2)
	}
	if !strings.Contains(strings.ToLower(out2), "blackout") {
		t.Fatalf("expected blackout confirmation, got: %s", out2)
	}
}

func TestCLICompetitionScoreboard(t *testing.T) {
	compID := createTestCompetition(t, "SBComp")
	_, stderr, code := runCLI(t, "competition", "scoreboard", compID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
}

func TestCLICompetitionScoreboardJSON(t *testing.T) {
	compID := createTestCompetition(t, "SBJSONComp")
	out, stderr, code := runCLI(t, "competition", "scoreboard", "--json", compID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("expected JSON array, got: %s err: %v", out, err)
	}
}

// ── Task 6: scoreboard freeze / unfreeze ─────────────────────────────────────

func TestCLIScoreboardFreeze(t *testing.T) {
	out, stderr, code := runCLI(t, "scoreboard", "freeze")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(out), "frozen") && !strings.Contains(strings.ToLower(out), "freeze") {
		t.Fatalf("expected freeze confirmation, got: %s", out)
	}
	// cleanup
	runCLI(t, "scoreboard", "unfreeze")
}

func TestCLIScoreboardUnfreeze(t *testing.T) {
	runCLI(t, "scoreboard", "freeze")
	out, stderr, code := runCLI(t, "scoreboard", "unfreeze")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(out), "frozen") && !strings.Contains(strings.ToLower(out), "unfreeze") {
		t.Fatalf("expected unfreeze confirmation, got: %s", out)
	}
}

// ── Task 7: submissions feed ─────────────────────────────────────────────────

func TestCLISubmissions(t *testing.T) {
	chID := createTestChallenge(t, "SubFeedCh")
	qID := createTestQuestion(t, chID, "SubQ", "flag{sub_test}", 100)
	// submit the correct flag
	runCLI(t, "flag", "submit", qID, "flag{sub_test}")

	out, stderr, code := runCLI(t, "submissions")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(out, "SubFeedCh") && !strings.Contains(out, "SubQ") {
		t.Fatalf("expected submission in feed, got: %s", out)
	}
}

func TestCLISubmissionsJSON(t *testing.T) {
	out, stderr, code := runCLI(t, "submissions", "--json")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("expected JSON array, got: %s err: %v", out, err)
	}
}

func TestCLISubmissionsCompetitionFilter(t *testing.T) {
	compID := createTestCompetition(t, "SubCompFilter")
	_, stderr, code := runCLI(t, "submissions", "--competition", compID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
}

func TestCLISubmissionsCompetitionFilterJSON(t *testing.T) {
	compID := createTestCompetition(t, "SubCompFilterJSON")
	out, stderr, code := runCLI(t, "submissions", "--competition", compID, "--json")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("expected JSON array, got: %s err: %v", out, err)
	}
}

// ── Task 8: user profile ──────────────────────────────────────────────────────

func TestCLIUserProfile(t *testing.T) {
	out, stderr, code := runCLI(t, "user", "profile")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(out, "Admin") {
		t.Fatalf("expected admin name in profile, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "rank") {
		t.Fatalf("expected 'rank' in output, got: %s", out)
	}
}

func TestCLIUserProfileJSON(t *testing.T) {
	out, stderr, code := runCLI(t, "user", "profile", "--json")
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("expected JSON object, got: %s err: %v", out, err)
	}
	if _, ok := m["name"]; !ok {
		t.Fatalf("expected 'name' field in JSON, got keys: %v", m)
	}
}

func TestCLIUserProfileByID(t *testing.T) {
	// Get admin user ID from user list
	var users []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	runCLIJSON(t, &users, "user", "list", "--json")
	var adminID string
	for _, u := range users {
		if u.Email == adminEmail {
			adminID = u.ID
			break
		}
	}
	if adminID == "" {
		t.Skip("could not find admin user ID")
	}

	out, stderr, code := runCLI(t, "user", "profile", adminID)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(out, "Admin") {
		t.Fatalf("expected name in profile by ID, got: %s", out)
	}
}

func TestCLIUserProfileMissingAuth(t *testing.T) {
	_, err := runCLIWithErrorAndNoAuth(t, "user", "profile")
	if err == nil {
		t.Fatal("expected error without auth")
	}
}

// Task 10: error handling & edge cases

func TestCLIQuestionUpdateInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "question", "update", "nonexistent-id",
		"--name", "x", "--flag", "f", "--points", "1")
	if code == 0 {
		t.Fatal("expected error for invalid question ID")
	}
}

func TestCLIHintUpdateInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "hint", "update", "nonexistent-id",
		"--content", "x", "--cost", "1", "--order", "1")
	if code == 0 {
		t.Fatal("expected error for invalid hint ID")
	}
}

func TestCLICompetitionUpdateInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "competition", "update", "99999", "--name", "x")
	if code == 0 {
		t.Fatal("expected error for invalid competition ID")
	}
}

func TestCLICompetitionBlackoutInvalidID(t *testing.T) {
	_, _, code := runCLI(t, "competition", "blackout", "99999")
	if code == 0 {
		t.Fatal("expected error for invalid competition ID")
	}
}

func TestCLICompetitionTeamsMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "competition", "teams")
	if code == 0 {
		t.Fatal("expected error for missing competition ID")
	}
}

func TestCLICompetitionScoreboardMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "competition", "scoreboard")
	if code == 0 {
		t.Fatal("expected error for missing competition ID")
	}
}

func TestCLIChallengeImportMissingArg(t *testing.T) {
	_, _, code := runCLI(t, "challenge", "import")
	if code == 0 {
		t.Fatal("expected error for missing file arg")
	}
}

func TestCLIChallengeImportBadFile(t *testing.T) {
	_, _, code := runCLI(t, "challenge", "import", "/nonexistent/path.json")
	if code == 0 {
		t.Fatal("expected error for missing file")
	}
}

func TestCLIConfigExportImport(t *testing.T) {
	// Setup: create category, difficulty, challenge, competition
	chID := createTestChallenge(t, "ConfigExportCh")
	createTestQuestion(t, chID, "ConfigQ", "flag{cfg}", 100)

	// Export config via API (authenticated)
	token := adminToken(t)
	exportReq, _ := http.NewRequest("GET", cliServer+"/api/admin/config/export", nil)
	exportReq.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	resp, err := http.DefaultClient.Do(exportReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var bundle struct {
		Version    int `json:"version"`
		Challenges []struct {
			Name string `json:"name"`
		} `json:"challenges"`
		Competitions []struct {
			Name string `json:"name"`
		} `json:"competitions"`
		SiteSettings map[string]string `json:"site_settings"`
	}
	if err := json.Unmarshal(body, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Version != 2 {
		t.Fatalf("expected version=2, got %d", bundle.Version)
	}
	found := false
	for _, ch := range bundle.Challenges {
		if ch.Name == "ConfigExportCh" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("challenge not found in config export")
	}
	if bundle.SiteSettings == nil {
		t.Fatal("site_settings missing from config export")
	}

	// Import via API (round-trip, authenticated)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "config.json")
	fw.Write(body)
	mw.Close()
	importReq, _ := http.NewRequest("POST", cliServer+"/api/admin/config/import", &buf)
	importReq.Header.Set("Content-Type", mw.FormDataContentType())
	importReq.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	resp2, err := http.DefaultClient.Do(importReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("import status %d: %s", resp2.StatusCode, b)
	}
}

func TestCLISubmissionsInvalidCompetition(t *testing.T) {
	_, _, code := runCLI(t, "submissions", "--competition", "99999")
	if code == 0 {
		t.Fatal("expected error for invalid competition ID")
	}
}

func TestCLIChallengeGetHintCount(t *testing.T) {
	chID := createTestChallenge(t, "HintCountCh")
	qID := createTestQuestion(t, chID, "HintCountQ", "flag{hc}", 100)
	createTestHint(t, qID, "hint one", 0)
	createTestHint(t, qID, "hint two", 10)

	resp, err := http.Get(cliServer + "/api/challenges/" + chID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Questions []struct {
			HintCount int `json:"hint_count"`
		} `json:"questions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Questions) == 0 {
		t.Fatal("no questions")
	}
	if env.Questions[0].HintCount != 2 {
		t.Fatalf("expected hint_count=2, got %d", env.Questions[0].HintCount)
	}
}

func TestCLISubmitLoopHintCountInLabel(t *testing.T) {
	// Verify hint_count is present on question via API (the CLI uses this to build the picker label)
	chID := createTestChallenge(t, "PickerHintCh")
	qID := createTestQuestion(t, chID, "PickerQ", "flag{ph}", 100)
	createTestHint(t, qID, "a hint", 0)

	resp, err := http.Get(cliServer + "/api/challenges/" + chID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Questions []struct {
			HintCount int `json:"hint_count"`
		} `json:"questions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Questions) == 0 {
		t.Fatal("no questions")
	}
	if env.Questions[0].HintCount != 1 {
		t.Fatalf("expected hint_count=1, got %d", env.Questions[0].HintCount)
	}
}

func TestCLIConfigExportCLI(t *testing.T) {
	// Export via CLI to stdout (JSON)
	stdout, stderr, code := runCLI(t, "config", "export")
	if code != 0 {
		t.Fatalf("config export failed: %s", stderr)
	}
	var bundle map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &bundle); err != nil {
		t.Fatalf("config export output is not valid JSON: %v", err)
	}
	v, ok := bundle["version"]
	if !ok {
		t.Fatal("config export missing 'version' field")
	}
	if int(v.(float64)) != 2 {
		t.Fatalf("expected version 2, got %v", v)
	}

	// Export to YAML file by extension
	yamlFile := filepath.Join(t.TempDir(), "config.yaml")
	_, stderr, code = runCLI(t, "config", "export", "--output", yamlFile)
	if code != 0 {
		t.Fatalf("config export --output yaml failed: %s", stderr)
	}
	yamlData, err := os.ReadFile(yamlFile)
	if err != nil {
		t.Fatalf("failed to read exported yaml: %v", err)
	}
	var yamlBundle map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &yamlBundle); err != nil {
		t.Fatalf("exported yaml is not valid YAML: %v", err)
	}

	// Import YAML file back
	stdout, stderr, code = runCLI(t, "config", "import", yamlFile)
	if code != 0 {
		t.Fatalf("config import yaml failed (stderr: %s, stdout: %s)", stderr, stdout)
	}
}
