# Browse + Submit Design

**Date**: 2026-03-11
**Status**: Approved

## Problem

`hctf2 challenge browse` lets you pick a challenge and view its detail, but you have to exit the TUI, copy the question ID, and run `flag submit <id> <flag>` manually. That's too many steps for the core participant loop.

## Goal

After selecting a challenge in browse, the CLI stays alive and lets you submit a flag without leaving the terminal.

## Flow

```
hctf2 challenge browse
  → [bubbletea] navigate list, press enter
  → challenge detail printed to stdout (existing)
  → [huh] if >1 question: Select picker — choose which question
  → [huh] Input — type the flag
  → submit via POST /api/questions/{id}/submit
  → print result: ✓ Correct! +N pts  or  ✗ Incorrect, try again
  → [huh] Confirm "Submit another flag?" → yes loops back, no exits
```

If the challenge has exactly one question the Select picker is skipped.

## Changes

| File | Change |
|------|--------|
| `internal/client/challenges.go` | Add `Question` struct; extend `GetChallenge` return to include questions from the existing `{"challenge":…,"questions":…}` envelope |
| `cmd/challenge.go` | After `runChallengeGet`, call new `runSubmitLoop(c, challengeID)` |
| `cmd/challenge.go` | `runSubmitLoop`: huh Select (>1 question) + huh Input + submit + confirm loop |

No new files, commands, or flags.

## Out of Scope

- Marking questions as solved/skipping already-solved ones (no API endpoint returns per-user solve status in the challenge detail)
- Full TUI state machine (bubbletea for the submit step)
- `--no-submit` flag to suppress the prompt
