# Known Issues

## Admin Dashboard: Edit/Delete Buttons on Dynamic Elements - FIXED

**Status**: Fixed
**Priority**: Medium
**Discovered**: 2026-02-10
**Fixed**: 2026-02-10
**Affects**: Admin dashboard (challenges, questions, and hints tabs)

### Problem

Edit and Delete buttons on newly created challenges/questions/hints didn't work until page refresh.

### Root Cause

Two issues working together:
1. Handler HTML fragments (in `challenges.go`) returned bare `<button>` tags with no event handlers
2. Template used `onclick="htmx.ajax(...)"` which doesn't get properly bound on dynamically inserted HTML, and `htmx.process()` only initializes `hx-*` attributes, not JavaScript handlers

### Fix

Replaced all `onclick`/`@click` patterns with declarative HTMX attributes (`hx-get`, `hx-delete`, `hx-confirm`) which are automatically processed by HTMX on dynamic elements. Also updated all Go handler HTML fragments to include the same HTMX attributes. Removed the now-unnecessary `htmx.process()` afterSwap calls from create forms.
