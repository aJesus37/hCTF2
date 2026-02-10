# Known Issues

## Admin Dashboard: Edit Button Not Working for Newly Created Items

**Status**: Documented for future fix
**Priority**: Medium
**Discovered**: 2026-02-10
**Affects**: Admin dashboard (challenges and questions tabs)

### Description

Edit buttons on newly created challenges/questions don't work until page refresh. The button click is detected (blue border appears on card) but the edit form doesn't appear.

### Root Cause

HTMX dynamic element processing issue. Onclick handlers with `htmx.ajax()` were implemented but still exhibit problems with newly created items. The issue appears to be that HTMX event handlers and processed attributes don't fully attach to dynamically created DOM elements.

### Reproduction Steps

1. Navigate to `/admin`
2. Click the "Challenges" tab
3. Create a new challenge using the form
4. Immediately click the "Edit" button (without page refresh)
5. **Expected**: Edit form appears below the challenge card
6. **Actual**: Button click registers (blue border highlights) but no edit form appears

Same reproduction applies to questions in the "Questions" tab.

### Files Involved

- `internal/views/templates/admin.html`
  - Lines 179-185: Challenge edit button
  - Lines 352-358: Question edit button
  - Current implementation: `onclick="htmx.ajax('GET', '/admin/challenges/{ID}/edit', {target: '#challenge-{ID}', swap: 'outerHTML'})"`

### Investigation Notes

- Edit buttons work correctly on pre-existing items after page load
- Edit buttons fail on newly created items (click detected but no form)
- Multiple approaches attempted:
  - `hx-get` attributes (failed - not processed on dynamic elements)
  - `htmx.process()` after creation (failed)
  - `onclick` with `htmx.ajax()` (current - still failing)
- Browser testing confirms issue (not a testing artifact)

### Workaround

Refresh the page after creating a challenge or question to enable edit functionality.

### Next Steps for Fix

1. Debug why `htmx.ajax()` in onclick doesn't trigger for dynamic elements
2. Test with HTMX debug mode enabled: `htmx.logAll()`
3. Review HTMX documentation on dynamic element handling
4. Consider alternatives:
   - Event delegation pattern for dynamically inserted elements
   - Direct fetch API + manual DOM manipulation
   - Re-initialize HTMX on newly inserted container
5. Check HTMX version and known issues in that version

### Additional Context

The challenge/question creation form includes:
```html
hx-on="htmx:afterSwap: setTimeout(() => {
  const elem = document.querySelector('#challenges-list').lastElementChild;
  if(elem) htmx.process(elem);
}, 10)"
```

This successfully appends the new element to the DOM, but subsequent onclick handlers don't work as expected.
