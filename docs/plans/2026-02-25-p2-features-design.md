# hCTF2 P2 Features Design

**Date**: 2026-02-25
**Status**: Approved
**Scope**: 7 remaining P2 open source readiness features

---

## 1. File Attachments

### Storage Interface

`internal/storage/storage.go`:
```go
type Storage interface {
    Upload(ctx context.Context, filename string, r io.Reader) (url string, err error)
    Delete(ctx context.Context, url string) error
}
```

Two backends:
- **LocalStorage**: writes to `--upload-dir` (default `./uploads`), serves files at `GET /uploads/{filename}` (authenticated users only)
- **S3Storage**: uses `aws-sdk-go-v2`, activated when `--s3-bucket` or `AWS_ACCESS_KEY_ID` is set; returns direct S3/R2/MinIO URLs

Auto-detection: if S3 config present → S3Storage, otherwise LocalStorage.

### Database

Migration 010: `file_url TEXT` column on `challenges` table (model field `*string` already exists).

### Routes

- `POST /api/admin/challenges/{id}/upload` — multipart, max 50MB, returns `{"url": "..."}`
- `DELETE /api/admin/challenges/{id}/file` — removes attachment and deletes from storage
- `GET /uploads/{filename}` — serves local files, requires authentication

### Admin UI

Challenge edit form: file upload input + current file display with filename and delete button.

**Validation**: agent-browser screenshot required.

---

## 2. Score Freezing

### Database

Migration 011: two new columns in `site_settings`:
- `freeze_enabled BOOLEAN DEFAULT 0`
- `freeze_at DATETIME NULL`

### Logic

When `freeze_enabled = 1`, scoreboard queries append:
```sql
AND s.created_at <= COALESCE(freeze_at, CURRENT_TIMESTAMP)
```
Flag submissions still accepted — only scoreboard display is frozen.

### Admin UI

New "Competition" section in Settings tab:
- Datetime picker for scheduled freeze time
- Manual freeze toggle (immediate override)
- Status indicator (frozen / live)

### API

`POST /api/admin/settings/freeze` — sets `freeze_at` and `freeze_enabled`.

**Validation**: agent-browser screenshot required.

---

## 3. Rate Limiting

### Implementation

`internal/ratelimit/ratelimit.go`:
- `Limiter` struct with `sync.Map` of `userID → *rate.Limiter`
- Each user: 5 requests/minute (configurable via `--submission-rate-limit`, default `5`)
- Background goroutine cleans idle limiters every 5 minutes

Uses `golang.org/x/time/rate` (already in stdlib-adjacent, no new external dep).

### Applied To

`POST /api/questions/{id}/submit` only.

### Response

HTTP 429 + HTMX-friendly fragment:
```html
<div class="text-red-400">Too many attempts. Please wait before trying again.</div>
```

---

## 4. CTFtime.org JSON Export

### Endpoint

`GET /api/ctftime` — public, no auth required.

### Format

```json
{
  "standings": [
    {"pos": 1, "team": "TeamName", "score": 1500},
    {"pos": 2, "team": "AnotherTeam", "score": 1200}
  ]
}
```

- Uses existing team scoreboard query
- Respects score freeze if active
- Returns HTTP 404 if no teams exist

### Admin UI

Settings page: display the CTFtime endpoint URL for copy-paste into CTFtime event config.

**Validation**: agent-browser screenshot required.

---

## 5. Challenge Import/Export

### Export

`GET /api/admin/export` — admin only, returns JSON file download.

Schema:
```json
{
  "version": 1,
  "exported_at": "2026-02-25T00:00:00Z",
  "categories": ["Web", "Crypto"],
  "difficulties": ["Easy", "Hard"],
  "challenges": [
    {
      "name": "Challenge Name",
      "description": "...",
      "category": "Web",
      "difficulty": "Easy",
      "dynamic_scoring": false,
      "initial_points": 100,
      "minimum_points": 50,
      "decay_threshold": 10,
      "file_url": "https://...",
      "questions": [
        {
          "name": "Part 1",
          "description": "...",
          "flag": "flag{...}",
          "flag_mask": "flag{...}",
          "points": 100,
          "hints": [
            {"content": "Try harder", "cost": 10, "order": 1}
          ]
        }
      ]
    }
  ]
}
```

File attachments export as URLs only (not re-uploaded).

### Import

`POST /api/admin/import` — multipart JSON file, admin only.

Behavior:
- Validates JSON structure
- Creates missing categories and difficulties
- Creates challenges, questions, hints
- **Duplicate name handling**: if challenge name exists, append `(2)`, `(3)`, etc. until unique
- Returns summary:
```json
{
  "imported": 5,
  "renamed": ["Web Challenge → Web Challenge (2)"],
  "errors": []
}
```

### Admin UI

Settings tab: Export button + Import file picker. After import, show result summary — renamed items appear in a yellow warning box.

**Validation**: agent-browser screenshot required.

---

## 6. Accessibility: Skip-to-Main Link

`internal/views/templates/base.html` — first element in `<body>`:
```html
<a href="#main-content"
   class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-purple-600 focus:text-white focus:rounded">
  Skip to main content
</a>
```

Add `id="main-content"` to `<main>` element. No backend changes.

**Validation**: agent-browser screenshot required.

---

## 7. Accessibility: Focus Trap in Modals

Small Alpine.js utility in `/static/js/focus-trap.js` (included in `base.html`):
- On modal open: find all focusable elements inside, focus first one
- Tab/Shift+Tab cycles within modal only
- On modal close: return focus to the trigger element

Applied to all Alpine.js `x-show` modals: teams modal, admin edit forms, hint unlock dialogs.

No backend changes.

**Validation**: agent-browser screenshot required.

---

## Implementation Order

1. Rate limiting (smallest, highest security value)
2. Score freezing (DB + backend + UI)
3. CTFtime export (backend only, simple)
4. File attachments (largest, storage interface)
5. Challenge import/export (depends on stable challenge schema)
6. Skip-to-main link (trivial, frontend only)
7. Focus trap in modals (frontend only)

---

## New Dependencies

- `aws-sdk-go-v2` packages (S3 storage, optional activation)
- `golang.org/x/time/rate` (rate limiting — already in Go extended stdlib)

---

## Migrations Required

- **010**: `file_url` on challenges (may already exist as column)
- **011**: `freeze_enabled`, `freeze_at` on site_settings
