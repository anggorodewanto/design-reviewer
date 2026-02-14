# Phase 19: Keyboard Shortcut to Post Comments

## Goal
Allow users to submit comments and replies by pressing Ctrl+Enter (or Cmd+Enter on macOS) instead of clicking the Post/Reply button.

## What to Change

### `web/static/annotations.js`

Add a `keydown` listener to each comment/reply `<textarea>` that triggers the adjacent submit button when the shortcut is pressed:

- In `showNewCommentForm()`: attach listener to `#nc-body` that clicks `#nc-submit` on Ctrl+Enter / Cmd+Enter.
- In `openPanel()`: attach listener to `#rp-body` that clicks `#rp-submit` on Ctrl+Enter / Cmd+Enter.

Detection logic:
```js
textarea.addEventListener("keydown", function (e) {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        submitBtn.click();
    }
});
```

### `web/templates/viewer.html` (optional)

Add a hint below each textarea: `<span class="shortcut-hint">Ctrl+Enter to post</span>` (display `⌘+Enter` on macOS via `navigator.platform` check).

### `web/static/style.css`

Style `.shortcut-hint` — small muted text below the textarea.

## Files to Change

- `web/static/annotations.js`
- `web/templates/viewer.html` (optional hint)
- `web/static/style.css` (optional hint style)

## Acceptance Criteria

- Pressing Ctrl+Enter in the new-comment textarea posts the comment
- Pressing Cmd+Enter on macOS in the new-comment textarea posts the comment
- Pressing Ctrl+Enter / Cmd+Enter in the reply textarea posts the reply
- Plain Enter still inserts a newline (no accidental submission)
