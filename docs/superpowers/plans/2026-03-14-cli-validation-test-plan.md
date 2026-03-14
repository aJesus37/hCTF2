# CLI Validation Test Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add integration tests for every CLI command added since the last test pass, validate output correctness, error handling, JSON flag, quiet flag, and web UI parity where applicable.

**Architecture:** All tests live in `cli_integration_test.go`. Each test starts the real binary against a fresh in-memory server (TestMain pattern already in place). Tests are non-interactive (no TTY), driving commands via flags. Parity checks call the REST API directly alongside the CLI and compare results.

**Tech Stack:** Go `testing`, `os/exec`, `net/http`, existing `runCLI` / `runCLIWithEnv` helpers, `encoding/json`.

---

## What already exists (do NOT re-test)

These commands have coverage — skip them:

- login, logout, status, register
- challenge list/get/create/update/delete/browse (requires TTY guard)
- question list/create/delete
- hint list/create/delete
- team list/get/create/join/leave/disband/invite-regen
- competition list/get/create/delete/start/end/add-challenge/remove-challenge/freeze/unfreeze/register
- user list/promote/demote/delete
- category list/create/delete
- difficulty list/create/delete
- flag submit (wrong flag case)
- scoreboard (global)

---

## Chunk 1: challenge update + new flags

### Task 1: challenge update — visible, min-points, decay flags

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIChallengeUpdateVisible(t *testing.T) {
    // create challenge, update with --visible, verify via API
    id := createTestChallenge(t, "VisTest")
    out := runCLI(t, "challenge", "update", id, "--title", "VisTest", "--category", "web",
        "--difficulty", "easy", "--points", "100", "--visible")
    if !strings.Contains(out, "Updated") {
        t.Fatalf("expected 'Updated', got: %s", out)
    }
    // verify via API
    ch := apiGetChallenge(t, id)
    if !ch.Visible {
        t.Fatal("expected visible=true after update")
    }
}

func TestCLIChallengeUpdateDynamicScoring(t *testing.T) {
    id := createTestChallenge(t, "DynTest")
    runCLI(t, "challenge", "update", id, "--title", "DynTest", "--category", "web",
        "--difficulty", "easy", "--points", "500", "--min-points", "50", "--decay", "10")
    ch := apiGetChallenge(t, id)
    if ch.MinimumPoints != 50 {
        t.Fatalf("expected min_points=50, got %d", ch.MinimumPoints)
    }
    if ch.DecayThreshold != 10 {
        t.Fatalf("expected decay=10, got %d", ch.DecayThreshold)
    }
}
```

Helper needed (add near top of test file if not present):
```go
type apiChallenge struct {
    ID             string `json:"id"`
    Visible        bool   `json:"visible"`
    MinimumPoints  int    `json:"minimum_points"`
    DecayThreshold int    `json:"decay_threshold"`
    InitialPoints  int    `json:"initial_points"`
}

func apiGetChallenge(t *testing.T, id string) apiChallenge {
    t.Helper()
    resp, err := http.Get(testServerURL + "/api/challenges/" + id)
    if err != nil { t.Fatal(err) }
    defer resp.Body.Close()
    var env struct{ Challenge apiChallenge `json:"challenge"` }
    if err := json.NewDecoder(resp.Body).Decode(&env); err != nil { t.Fatal(err) }
    return env.Challenge
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/jesus/Projects/hCTF2
go test -run "TestCLIChallengeUpdateVisible|TestCLIChallengeUpdateDynamicScoring" -v ./... 2>&1 | tail -20
```

Expected: FAIL (helpers missing or assertions fail)

- [ ] **Step 3: Implement helpers + make tests pass**

Add `apiGetChallenge` helper and `createTestChallenge` if not already present (check — it likely already exists). Run tests again.

- [ ] **Step 4: Verify pass**

```bash
go test -run "TestCLIChallengeUpdateVisible|TestCLIChallengeUpdateDynamicScoring" -v ./... 2>&1 | tail -10
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cli_integration_test.go
git commit -m "test(cli): challenge update --visible and --min-points/--decay flags"
```

---

### Task 2: challenge export / import round-trip

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIChallengeExport(t *testing.T) {
    createTestChallenge(t, "ExportMe")
    out := runCLI(t, "challenge", "export")
    // must be valid JSON array
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("export output is not valid JSON array: %v\ngot: %s", err, out)
    }
    if len(items) == 0 {
        t.Fatal("expected at least one challenge in export")
    }
}

func TestCLIChallengeExportToFile(t *testing.T) {
    createTestChallenge(t, "ExportFile")
    f := t.TempDir() + "/export.json"
    runCLI(t, "challenge", "export", "--output", f)
    data, err := os.ReadFile(f)
    if err != nil { t.Fatalf("file not written: %v", err) }
    var items []json.RawMessage
    if err := json.Unmarshal(data, &items); err != nil {
        t.Fatalf("file content is not valid JSON: %v", err)
    }
}

func TestCLIChallengeImport(t *testing.T) {
    // export, then import into a fresh state by checking count increases
    before := runCLI(t, "challenge", "list", "--json")
    var beforeList []json.RawMessage
    json.Unmarshal([]byte(before), &beforeList)

    exported := runCLI(t, "challenge", "export")
    f := t.TempDir() + "/import.json"
    os.WriteFile(f, []byte(exported), 0644)

    out := runCLI(t, "challenge", "import", f)
    if !strings.Contains(out, "Imported") {
        t.Fatalf("expected 'Imported', got: %s", out)
    }
}

func TestCLIChallengeExportJSON(t *testing.T) {
    // --json flag on export produces same result as raw export (both are JSON arrays)
    out := runCLI(t, "challenge", "export", "--json")
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("--json export not valid JSON: %v", err)
    }
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run "TestCLIChallengeExport|TestCLIChallengeImport" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Fix and make pass**

- [ ] **Step 4: Verify pass**

```bash
go test -run "TestCLIChallengeExport|TestCLIChallengeImport" -v ./... 2>&1 | tail -10
```

- [ ] **Step 5: Commit**

```bash
git add cli_integration_test.go
git commit -m "test(cli): challenge export/import round-trip"
```

---

## Chunk 2: question update + hint update

### Task 3: question update

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIQuestionUpdate(t *testing.T) {
    chID := createTestChallenge(t, "QUpdateCh")
    qID := createTestQuestion(t, chID, "OldName", "flag{old}", 100)

    out := runCLI(t, "question", "update", qID,
        "--name", "NewName", "--flag", "flag{new}", "--points", "200")
    if !strings.Contains(out, "Updated") {
        t.Fatalf("expected 'Updated', got: %s", out)
    }
    // verify via API
    q := apiGetQuestion(t, chID)
    if q.Name != "NewName" {
        t.Fatalf("expected name=NewName, got %s", q.Name)
    }
}

func TestCLIQuestionUpdateCaseSensitive(t *testing.T) {
    chID := createTestChallenge(t, "QCaseCh")
    qID := createTestQuestion(t, chID, "CaseQ", "flag{case}", 50)
    runCLI(t, "question", "update", qID,
        "--name", "CaseQ", "--flag", "flag{case}", "--points", "50", "--case-sensitive")
    // no error = pass; case-sensitive flag accepted
}

func TestCLIQuestionUpdateMissingArg(t *testing.T) {
    out, err := runCLIWithError(t, "question", "update")
    if err == nil {
        t.Fatalf("expected error for missing arg, got: %s", out)
    }
}
```

Helper:
```go
type apiQuestion struct{ Name string `json:"name"` }

func apiGetQuestion(t *testing.T, challengeID string) apiQuestion {
    t.Helper()
    resp, err := http.Get(testServerURL + "/api/challenges/" + challengeID)
    if err != nil { t.Fatal(err) }
    defer resp.Body.Close()
    var env struct{ Questions []apiQuestion `json:"questions"` }
    json.NewDecoder(resp.Body).Decode(&env)
    if len(env.Questions) == 0 { t.Fatal("no questions found") }
    return env.Questions[0]
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLIQuestionUpdate" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Implement and pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): question update command"
```

---

### Task 4: hint update

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIHintUpdate(t *testing.T) {
    chID := createTestChallenge(t, "HintUpdateCh")
    qID := createTestQuestion(t, chID, "HintQ", "flag{h}", 100)
    hID := createTestHint(t, qID, "old hint", 10)

    out := runCLI(t, "hint", "update", hID,
        "--content", "new hint text", "--cost", "20", "--order", "1")
    if !strings.Contains(out, "Updated") {
        t.Fatalf("expected 'Updated', got: %s", out)
    }
}

func TestCLIHintUpdateMissingArg(t *testing.T) {
    out, err := runCLIWithError(t, "hint", "update")
    if err == nil {
        t.Fatalf("expected error, got: %s", out)
    }
}
```

Helper (if not present):
```go
func createTestHint(t *testing.T, questionID, content string, cost int) string {
    t.Helper()
    out := runCLI(t, "hint", "create",
        "--question", questionID,
        "--content", content,
        "--cost", strconv.Itoa(cost),
        "--quiet")
    return strings.TrimSpace(out)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLIHintUpdate" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Make pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): hint update command"
```

---

## Chunk 3: competition new commands

### Task 5: competition update + teams + blackout + scoreboard

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLICompetitionUpdate(t *testing.T) {
    compID := createTestCompetition(t, "UpdateComp")
    out := runCLI(t, "competition", "update", compID,
        "--name", "UpdatedComp", "--description", "new desc")
    if !strings.Contains(out, "Updated") {
        t.Fatalf("expected 'Updated', got: %s", out)
    }
}

func TestCLICompetitionUpdateMissingArg(t *testing.T) {
    _, err := runCLIWithError(t, "competition", "update")
    if err == nil { t.Fatal("expected error") }
}

func TestCLICompetitionTeams(t *testing.T) {
    compID := createTestCompetition(t, "TeamsComp")
    // no teams registered yet — should not error, just empty
    out := runCLI(t, "competition", "teams", compID)
    // output is either a table header or empty message — just verify no error
    _ = out
}

func TestCLICompetitionTeamsJSON(t *testing.T) {
    compID := createTestCompetition(t, "TeamsJSONComp")
    out := runCLI(t, "competition", "teams", "--json", compID)
    // must be valid JSON (array)
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("expected JSON array, got: %s", out)
    }
}

func TestCLICompetitionBlackout(t *testing.T) {
    compID := createTestCompetition(t, "BlackoutComp")
    out := runCLI(t, "competition", "blackout", compID)
    if !strings.Contains(out, "blackout") && !strings.Contains(out, "Blackout") {
        t.Fatalf("expected confirmation, got: %s", out)
    }
    out2 := runCLI(t, "competition", "unblackout", compID)
    if !strings.Contains(out2, "blackout") && !strings.Contains(out2, "Blackout") {
        t.Fatalf("expected confirmation, got: %s", out2)
    }
}

func TestCLICompetitionScoreboard(t *testing.T) {
    compID := createTestCompetition(t, "SBComp")
    out := runCLI(t, "competition", "scoreboard", compID)
    // empty scoreboard — should not error
    _ = out
}

func TestCLICompetitionScoreboardJSON(t *testing.T) {
    compID := createTestCompetition(t, "SBJSONComp")
    out := runCLI(t, "competition", "scoreboard", "--json", compID)
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("expected JSON array, got: %s", out)
    }
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLICompetitionUpdate|TestCLICompetitionTeams|TestCLICompetitionBlackout|TestCLICompetitionScoreboard" -v ./... 2>&1 | tail -30
```

- [ ] **Step 3: Make pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): competition update, teams, blackout, scoreboard commands"
```

---

## Chunk 4: scoreboard freeze + submissions + user profile

### Task 6: scoreboard freeze / unfreeze

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIScoreboardFreeze(t *testing.T) {
    out := runCLI(t, "scoreboard", "freeze")
    if !strings.Contains(strings.ToLower(out), "freeze") {
        t.Fatalf("expected freeze confirmation, got: %s", out)
    }
}

func TestCLIScoreboardUnfreeze(t *testing.T) {
    runCLI(t, "scoreboard", "freeze")
    out := runCLI(t, "scoreboard", "unfreeze")
    if !strings.Contains(strings.ToLower(out), "freeze") {
        t.Fatalf("expected unfreeze confirmation, got: %s", out)
    }
}

func TestCLIScoreboardFreezeQuiet(t *testing.T) {
    out := runCLI(t, "scoreboard", "freeze", "--quiet")
    if strings.TrimSpace(out) != "" {
        t.Fatalf("expected empty output with --quiet, got: %s", out)
    }
    runCLI(t, "scoreboard", "unfreeze") // cleanup
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLIScoreboardFreeze" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Make pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): scoreboard freeze/unfreeze"
```

---

### Task 7: submissions feed

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLISubmissions(t *testing.T) {
    // create a challenge, question, submit a flag, then check feed
    chID := createTestChallenge(t, "SubFeedCh")
    qID := createTestQuestion(t, chID, "SubQ", "flag{sub_test}", 100)
    runCLI(t, "flag", "submit", qID, "flag{sub_test}")

    out := runCLI(t, "submissions")
    if !strings.Contains(out, "SubQ") && !strings.Contains(out, "SubFeedCh") {
        t.Fatalf("expected submission in feed, got: %s", out)
    }
}

func TestCLISubmissionsJSON(t *testing.T) {
    out := runCLI(t, "submissions", "--json")
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("expected JSON array, got: %s\nerr: %v", out, err)
    }
}

func TestCLISubmissionsCompetitionFilter(t *testing.T) {
    compID := createTestCompetition(t, "SubCompFilter")
    out := runCLI(t, "submissions", "--competition", compID)
    // no submissions yet — no error expected
    _ = out
}

func TestCLISubmissionsCompetitionFilterJSON(t *testing.T) {
    compID := createTestCompetition(t, "SubCompFilterJSON")
    out := runCLI(t, "submissions", "--competition", compID, "--json")
    var items []json.RawMessage
    if err := json.Unmarshal([]byte(out), &items); err != nil {
        t.Fatalf("expected JSON array, got: %s", out)
    }
}
```

Note: `--watch` flag requires TTY and time, so it is intentionally excluded from automated tests (like `challenge browse`).

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLISubmissions" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Make pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): submissions feed command"
```

---

### Task 8: user profile

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCLIUserProfile(t *testing.T) {
    // own profile (no arg)
    out := runCLI(t, "user", "profile")
    if !strings.Contains(out, "Admin") {
        t.Fatalf("expected admin name in profile, got: %s", out)
    }
    // should contain rank/points/solves rows
    if !strings.Contains(strings.ToLower(out), "rank") {
        t.Fatalf("expected 'rank' in output, got: %s", out)
    }
}

func TestCLIUserProfileJSON(t *testing.T) {
    out := runCLI(t, "user", "profile", "--json")
    var m map[string]interface{}
    if err := json.Unmarshal([]byte(out), &m); err != nil {
        t.Fatalf("expected JSON object, got: %s\nerr: %v", out, err)
    }
    if _, ok := m["name"]; !ok {
        t.Fatalf("expected 'name' field in JSON, got: %s", out)
    }
}

func TestCLIUserProfileByID(t *testing.T) {
    // get own user ID from status, then fetch by ID
    statusOut := runCLI(t, "status", "--json")
    var status struct{ ID string `json:"id"` }
    json.Unmarshal([]byte(statusOut), &status)
    if status.ID == "" { t.Skip("could not get user ID from status") }

    out := runCLI(t, "user", "profile", status.ID)
    if !strings.Contains(out, "Admin") {
        t.Fatalf("expected name in profile by ID, got: %s", out)
    }
}

func TestCLIUserProfileMissingAuth(t *testing.T) {
    // run without being logged in — should error
    out, err := runCLIWithErrorAndNoAuth(t, "user", "profile")
    if err == nil {
        t.Fatalf("expected error without auth, got: %s", out)
    }
}
```

Note: `runCLIWithErrorAndNoAuth` runs the CLI with a blank config (no token). Check if this helper exists; if not, add it using a temp config file with only the server URL.

- [ ] **Step 2: Run to verify failure**

```bash
go test -run "TestCLIUserProfile" -v ./... 2>&1 | tail -20
```

- [ ] **Step 3: Make pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "test(cli): user profile command"
```

---

## Chunk 5: web UI parity spot-checks

These are not automated Go tests — they are manual steps to run once against a live server to confirm the CLI and web UI agree on data.

### Task 9: manual parity validation checklist

Run each step against the server at `http://localhost:8090` with admin credentials.

- [ ] **challenge list vs web /challenges**
  ```bash
  ./hctf2 challenge list --server http://localhost:8090 --json | python3 -m json.tool | grep '"name"'
  # Compare count and names with /challenges in browser
  ```

- [ ] **challenge get vs web /challenges/{id}**
  ```bash
  ID=$(./hctf2 challenge list --server http://localhost:8090 --json | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['id'])")
  ./hctf2 challenge get --server http://localhost:8090 $ID
  # Verify title, category, difficulty, points match web UI challenge card
  ```

- [ ] **scoreboard vs web /scoreboard**
  ```bash
  ./hctf2 scoreboard --server http://localhost:8090
  # Compare top 3 entries with /scoreboard in browser (rank, user, score)
  ```

- [ ] **submissions vs web /submissions**
  ```bash
  ./hctf2 submissions --server http://localhost:8090 --json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'{len(d)} submissions')"
  # Compare count with /submissions page in browser
  ```

- [ ] **team list vs web /teams**
  ```bash
  ./hctf2 team list --server http://localhost:8090 --json | python3 -m json.tool
  # Verify team names and invite codes match web UI
  ```

- [ ] **competition list vs web /competitions**
  ```bash
  ./hctf2 competition list --server http://localhost:8090 --json | python3 -m json.tool
  # Verify competition names, statuses match web UI
  ```

- [ ] **user profile vs web /profile**
  ```bash
  ./hctf2 user profile --server http://localhost:8090
  # Verify name, points, solves match /profile page in browser
  ```

- [ ] **challenge export roundtrip**
  ```bash
  ./hctf2 challenge export --server http://localhost:8090 --output /tmp/export_check.json
  cat /tmp/export_check.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'{len(d)} challenges exported')"
  ```

- [ ] **Commit parity checklist results** (note any discrepancies found as issues in git commit message)

---

## Chunk 6: error handling & edge cases

### Task 10: missing args, wrong IDs, unauthorized access

**Files:**
- Modify: `cli_integration_test.go`

- [ ] **Step 1: Write failing tests**

```go
// question update
func TestCLIQuestionUpdateInvalidID(t *testing.T) {
    _, err := runCLIWithError(t, "question", "update", "nonexistent-id",
        "--name", "x", "--flag", "f", "--points", "1")
    if err == nil { t.Fatal("expected error for invalid question ID") }
}

// hint update
func TestCLIHintUpdateInvalidID(t *testing.T) {
    _, err := runCLIWithError(t, "hint", "update", "nonexistent-id",
        "--content", "x", "--cost", "1", "--order", "1")
    if err == nil { t.Fatal("expected error for invalid hint ID") }
}

// competition commands with invalid ID
func TestCLICompetitionUpdateInvalidID(t *testing.T) {
    _, err := runCLIWithError(t, "competition", "update", "99999", "--name", "x")
    if err == nil { t.Fatal("expected error for invalid competition ID") }
}

func TestCLICompetitionBlackoutInvalidID(t *testing.T) {
    _, err := runCLIWithError(t, "competition", "blackout", "99999")
    if err == nil { t.Fatal("expected error for invalid competition ID") }
}

func TestCLICompetitionTeamsMissingArg(t *testing.T) {
    _, err := runCLIWithError(t, "competition", "teams")
    if err == nil { t.Fatal("expected error for missing competition ID") }
}

func TestCLICompetitionScoreboardMissingArg(t *testing.T) {
    _, err := runCLIWithError(t, "competition", "scoreboard")
    if err == nil { t.Fatal("expected error for missing competition ID") }
}

// challenge export/import errors
func TestCLIChallengeImportMissingArg(t *testing.T) {
    _, err := runCLIWithError(t, "challenge", "import")
    if err == nil { t.Fatal("expected error for missing file arg") }
}

func TestCLIChallengeImportBadFile(t *testing.T) {
    _, err := runCLIWithError(t, "challenge", "import", "/nonexistent/path.json")
    if err == nil { t.Fatal("expected error for missing file") }
}

// submissions with invalid competition ID
func TestCLISubmissionsInvalidCompetition(t *testing.T) {
    _, err := runCLIWithError(t, "submissions", "--competition", "99999")
    if err == nil { t.Fatal("expected error for invalid competition ID") }
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test -run "TestCLIQuestionUpdateInvalidID|TestCLIHintUpdateInvalidID|TestCLICompetitionUpdateInvalidID|TestCLICompetitionBlackout|TestCLICompetitionTeamsMissing|TestCLICompetitionScoreboardMissing|TestCLIChallengeImport|TestCLISubmissionsInvalid" -v ./... 2>&1 | tail -30
```

- [ ] **Step 3: Fix any CLI bugs revealed and make tests pass**

If a test fails because the CLI doesn't return a non-zero exit code on error, fix the command to return `err` instead of printing and returning nil.

- [ ] **Step 4: Run full test suite to check for regressions**

```bash
go test ./... -v 2>&1 | grep -E "PASS|FAIL|---" | tail -30
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cli_integration_test.go
git commit -m "test(cli): error handling and edge cases for new commands"
```

---

## Final step: full test run

- [ ] **Run entire test suite**

```bash
cd /home/jesus/Projects/hCTF2
go test ./... -v -timeout 120s 2>&1 | tail -40
```

Expected: all tests PASS, no regressions.

- [ ] **Commit any remaining fixes**

```bash
git add -A
git commit -m "test(cli): full CLI validation test suite complete"
```
