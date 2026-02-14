# Phase 26: Attribute Injection XSS in sharing.js

**Severity:** 🟠 High

## Problem

The `esc()` helper escapes `<`, `>`, `&` but NOT `"` or `'`. When the result is placed inside an HTML attribute, a value containing a double quote breaks out of the attribute:

```js
// VULNERABLE — esc() doesn't escape quotes
'<button class="btn-remove" data-email="' + esc(m.email) + '">'
```

This affects both `sharing.js` and `annotations.js`.

## Fix

Update `esc()` in both files to also escape quotes:

```js
function esc(s) {
    var d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
```

## Files to Change

- `web/static/sharing.js`
- `web/static/annotations.js`

## Acceptance Criteria

- Double quotes and single quotes in user input are escaped in HTML attributes
- The `esc()` function produces `&quot;` and `&#39;` for quote characters
- Existing annotation and sharing UI still renders correctly
