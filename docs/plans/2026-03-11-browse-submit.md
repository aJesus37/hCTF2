# Browse + Submit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** After selecting a challenge in `hctf2 challenge browse`, the CLI stays alive and lets you pick a question (via `huh.Select` when there are multiple) and submit a flag without leaving the terminal.

**Architecture:** Extend the `GetChallenge` client call to also return the questions already present in the response envelope. Add a `runSubmitLoop` function in `cmd/challenge.go` that uses `huh` forms for question selection and flag entry, looping until the user declines to retry. Wire it into `runChallengeBrowse` after the detail printout.

**Tech Stack:** Go, `github.com/charmbracelet/huh` (already imported in `cmd/challenge.go`), existing `client.SubmitFlag`.

---

### Task 1: Expose questions from GetChallenge

**Files:**
- Modify: `internal/client/challenges.go`

The `GET /api/challenges/{id}` response already contains a `questions` array.
Currently `GetChallenge` discards it. We need a `Question` struct and a new
`GetChallengeWithQuestions` function (keep the old one for JSON output).

**Step 1: Write the failing test**

Add to `cli_integration_test.go` inside the `TestCLIChallengeGetJSON` test (or as a new
`TestCLIGetChallengeWithQuestions` unit test in `internal/client/`).

Create `internal/client/challenges_test.go`:

```go
package client_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/ajesus37/hCTF2/internal/client"
)

func TestGetChallengeWithQuestionsDecodesQuestions(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "challenge": map[string]any{
                "id": "ch-1", "name": "Web 101", "category": "web",
                "difficulty": "easy", "initial_points": 100,
            },
            "questions": []map[string]any{
                {"id": "q-1", "name": "Part 1", "flag_mask": "flag{***}", "points": 50},
                {"id": "q-2", "name": "Part 2", "flag_mask": "flag{***}", "points": 50},
            },
        })
    }))
    defer srv.Close()

    c := client.New(srv.URL, "token")
    ch, qs, err := c.GetChallengeWithQuestions("ch-1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ch.Title != "Web 101" {
        t.Errorf("expected title 'Web 101', got %q", ch.Title)
    }
    if len(qs) != 2 {
        t.Fatalf("expected 2 questions, got %d", len(qs))
    }
    if qs[0].ID != "q-1" || qs[1].ID != "q-2" {
        t.Errorf("unexpected question IDs: %v %v", qs[0].ID, qs[1].ID)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/client/ -run TestGetChallengeWithQuestionsDecodesQuestions -v
```
Expected: `FAIL — GetChallengeWithQuestions undefined`

**Step 3: Add `Question` struct and `GetChallengeWithQuestions` to `internal/client/challenges.go`**

Add after the `Challenge` struct:

```go
type Question struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    FlagMask string `json:"flag_mask"`
    Points   int    `json:"points"`
}
```

Add after `GetChallenge`:

```go
// GetChallengeWithQuestions returns the challenge and its questions.
func (c *Client) GetChallengeWithQuestions(id string) (*Challenge, []Question, error) {
    req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/challenges/%s", c.ServerURL, id), nil)
    resp, err := c.Do(req)
    if err != nil {
        return nil, nil, err
    }
    var envelope struct {
        Challenge Challenge  `json:"challenge"`
        Questions []Question `json:"questions"`
    }
    if err := decodeJSON(resp, &envelope); err != nil {
        return nil, nil, err
    }
    return &envelope.Challenge, envelope.Questions, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/client/ -run TestGetChallengeWithQuestionsDecodesQuestions -v
```
Expected: `PASS`

**Step 5: Commit**

```bash
git add internal/client/challenges.go internal/client/challenges_test.go
git commit -m "feat(client): add Question struct and GetChallengeWithQuestions"
```

---

### Task 2: Add `runSubmitLoop` to cmd/challenge.go

**Files:**
- Modify: `cmd/challenge.go`

This function receives a `*client.Client` and a challenge ID, fetches the challenge+questions, shows the picker (if >1 question), prompts for the flag, submits, prints the result, and loops.

**Step 1: Write the failing integration test**

Add to `cli_integration_test.go`. The test creates a challenge + question via API, then uses `runCLI` to verify the browse command now exits cleanly even when stdin is not a TTY (the submit loop detects non-TTY and skips the prompt, as `challenge browse` itself already gates on TTY).

The test for the submit loop behaviour itself is covered by the existing `TestCLIFlagSubmitWrongFlag` test plus a new test that exercises the huh path cannot run in non-TTY. This is the correct scope — `huh` forms require a TTY and cannot be tested headlessly.

Add this test:

```go
func TestCLIChallengeBrowseExitsCleanlyWithNoTTY(t *testing.T) {
    // browse must still fail cleanly (non-TTY guard) — the submit loop
    // is never reached because browse itself requires a TTY.
    _, _, code := runCLI(t, "challenge", "browse")
    assertError(t, "", "", code)
}
```

(This test already exists as `TestCLIChallengeBrowseRequiresTTY` — no new test needed.
The submit loop is TTY-only and integration-tested manually.)

**Step 2: Run existing tests to confirm baseline**

```bash
go test -run TestCLIChallengeBrowseRequiresTTY -count=1 -timeout 30s . -v
```
Expected: `PASS`

**Step 3: Add `runSubmitLoop` to `cmd/challenge.go`**

Add this function after `runChallengeBrowse`:

```go
// runSubmitLoop prompts the user to pick a question and submit a flag,
// looping until they decline. Silently returns if stdin is not a TTY.
func runSubmitLoop(c *client.Client, challengeID string) error {
    if !term.IsTerminal(int(os.Stdin.Fd())) {
        return nil
    }

    _, questions, err := c.GetChallengeWithQuestions(challengeID)
    if err != nil {
        return err
    }
    if len(questions) == 0 {
        fmt.Fprintln(os.Stdout, tui.MutedStyle.Render("No questions available."))
        return nil
    }

    for {
        // Pick a question (skip picker if only one).
        questionID := questions[0].ID
        questionName := questions[0].Name
        if len(questions) > 1 {
            opts := make([]huh.Option[string], len(questions))
            for i, q := range questions {
                label := fmt.Sprintf("%s  (%s, %d pts)", q.Name, q.FlagMask, q.Points)
                opts[i] = huh.NewOption(label, q.ID)
            }
            if err := huh.NewForm(huh.NewGroup(
                huh.NewSelect[string]().
                    Title("Which question?").
                    Options(opts...).
                    Value(&questionID),
            )).Run(); err != nil {
                return err
            }
            for _, q := range questions {
                if q.ID == questionID {
                    questionName = q.Name
                    break
                }
            }
        }

        // Prompt for flag.
        var flag string
        if err := huh.NewForm(huh.NewGroup(
            huh.NewInput().
                Title(fmt.Sprintf("Flag for %q", questionName)).
                Placeholder("flag{...}").
                Value(&flag),
        )).Run(); err != nil {
            return err
        }

        if flag == "" {
            return nil
        }

        // Submit.
        result, err := c.SubmitFlag(questionID, flag)
        if err != nil {
            fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render(fmt.Sprintf("Submit error: %v", err)))
        } else if result.Correct {
            fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render("✓ Correct!"))
        } else {
            fmt.Fprintln(os.Stdout, tui.ErrorStyle.Render("✗ Incorrect, try again"))
        }

        // Ask to continue.
        var again bool
        if err := huh.NewForm(huh.NewGroup(
            huh.NewConfirm().
                Title("Submit another flag?").
                Value(&again),
        )).Run(); err != nil || !again {
            return err
        }
    }
}
```

Also add `"github.com/ajesus37/hCTF2/internal/client"` to the import block in `cmd/challenge.go` if not already present (it's used indirectly via `newClient()` — you may need to add the direct import for the `*client.Client` parameter type).

**Step 4: Wire `runSubmitLoop` into `runChallengeBrowse`**

Replace the last few lines of `runChallengeBrowse`:

```go
// Before:
if id == "" {
    return nil
}
return runChallengeGet(nil, []string{id})

// After:
if id == "" {
    return nil
}
if err := runChallengeGet(nil, []string{id}); err != nil {
    return err
}
return runSubmitLoop(c, id)
```

**Step 5: Build to confirm it compiles**

```bash
go build ./...
```
Expected: no errors.

**Step 6: Run full test suite**

```bash
go test -timeout 120s -count=1 ./...
```
Expected: all tests pass (the submit loop is TTY-only, existing browse test still passes).

**Step 7: Commit**

```bash
git add cmd/challenge.go
git commit -m "feat(cli): add interactive flag submission to challenge browse"
```

---

### Task 3: Manual smoke test

**Step 1: Rebuild**

```bash
task rebuild
```

**Step 2: Start server (if not running)**

```bash
./hctf2 serve --port 8090 --dev &
```

**Step 3: Login**

```bash
./hctf2 login --server http://localhost:8090 --email admin@hctf.local --password changeme
```

**Step 4: Run browse and submit a flag**

```bash
./hctf2 challenge browse
```

Expected flow:
1. Bubbletea list appears — navigate with ↑/↓, press enter on a challenge
2. Challenge detail prints (title, category, difficulty, points, description)
3. If >1 question: `huh` Select picker appears — choose a question
4. `huh` Input appears for the flag — type and press enter
5. Result prints: `✓ Correct!` or `✗ Incorrect, try again`
6. `huh` Confirm "Submit another flag?" — yes loops, no exits

**Step 5: Push**

```bash
git push origin feat/cli-full
```
