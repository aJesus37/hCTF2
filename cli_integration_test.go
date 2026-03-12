package main

// CLI integration tests — exercise every command, subcommand, and flag.
//
// Strategy: TestMain builds the binary once, starts hctf2 serve as a
// subprocess on a free port, logs in as admin, and stores the server URL
// and config-file path in package-level variables.  Each test then calls
// runCLI() which execs the binary with HCTF2_CONFIG pointing at the temp
// config.  The suite is self-contained; no mock servers needed.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			serverProc.Process.Kill()
			serverProc.Wait()
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
	cmd.Run()

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
