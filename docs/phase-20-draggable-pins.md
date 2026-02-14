# Phase 20: Draggable Comment Pins

## Goal
Allow collaborators to drag existing comment pins to reposition them on the design, updating the stored coordinates via API.

## What to Build

### 1. New API Endpoint (`internal/api/comments.go`)

**`PATCH /api/comments/{id}/move`**
- Request body:
```json
{
  "x_percent": 55.3,
  "y_percent": 42.1
}
```
- Validates that `x_percent` and `y_percent` are between 0 and 100.
- Calls `db.MoveComment(id, xPercent, yPercent)`.
- Returns `{"ok": true}`.

Register the route in `Handler.RegisterRoutes()`.

### 2. Database Method (`internal/db/db.go`)

Add `MoveComment(id string, x, y float64) error`:
```go
func (d *DB) MoveComment(id string, x, y float64) error {
    _, err := d.Exec("UPDATE comments SET x_percent=?, y_percent=? WHERE id=?", x, y, id)
    return err
}
```

### 3. Drag Interaction (`web/static/annotations.js`)

Make pin markers draggable via mouse events on the overlay:

- On `mousedown` on a `.pin-marker`: record the pin element and starting mouse position. Prevent the pin's click handler from firing.
- On `mousemove` on the overlay: reposition the pin element to follow the cursor.
- On `mouseup`: calculate new `x_percent` / `y_percent` from the drop position relative to the overlay, then `PATCH /api/comments/{id}/move`. Reload comments on success.
- Only start dragging after a small movement threshold (e.g. 4px) to distinguish clicks from drags.

### 4. Styles (`web/static/style.css`)

- `.pin-marker` gets `cursor: grab`.
- While dragging, add `.pin-dragging` class with `cursor: grabbing` and `opacity: 0.7`.

## Files to Change

- `internal/api/comments.go` — new move endpoint
- `internal/db/db.go` — `MoveComment` method
- `web/static/annotations.js` — drag interaction
- `web/static/style.css` — drag cursor styles

## Acceptance Criteria

- Dragging a pin repositions it visually during the drag
- Dropping a pin saves the new coordinates via API
- Clicking a pin (without dragging) still opens the comment panel
- Pin positions persist after page reload
- Coordinates stay clamped within 0–100%
