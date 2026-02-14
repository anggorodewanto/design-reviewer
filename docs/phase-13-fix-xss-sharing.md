# Phase 13: Fix XSS in Sharing UI

## Problem

`web/static/sharing.js` line 47 injects `m.email` directly into `innerHTML` without escaping:

```js
membersList.innerHTML = members.map(m =>
    '<div class="member-row"><span>' + m.email + '</span>' +
    (window.isOwner ? '<button class="btn-remove" data-email="' + m.email + '">Remove</button>' : '') +
    '</div>'
).join('');
```

A crafted email address containing HTML/JS (e.g. `<img src=x onerror=alert(1)>`) would execute arbitrary JavaScript in the context of any user viewing the members list.

## Fix

Use the same `esc()` helper from `annotations.js`, or build DOM elements with `textContent` instead of string concatenation with `innerHTML`.

Both `m.email` occurrences need escaping — the display text and the `data-email` attribute.

## Files to Change

- `web/static/sharing.js`

## Acceptance Criteria

- Member emails render as plain text, never as HTML
- Injected HTML in email fields is displayed escaped, not executed
