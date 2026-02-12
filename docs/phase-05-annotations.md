# Phase 5: Annotation System

## Goal
Build the full pin-comment annotation system — clicking on a design drops a pin, users can comment, reply, resolve, and filter.

## Prerequisites
Phases 1-4 complete — design viewer page with iframe rendering works.

## What to Build

### 1. Comment API Endpoints (`internal/api/comments.go`)

**`GET /api/versions/{id}/comments`**
- Returns all comments for a version, including carried-over unresolved comments from previous versions
- Use `db.GetUnresolvedCommentsUpTo(versionID)` to get the full set
- Each comment includes its replies
- Response format:
```json
[
  {
    "id": "uuid",
    "version_id": "uuid",
    "page": "index.html",
    "x_percent": 45.2,
    "y_percent": 30.1,
    "author_name": "Jane Doe",
    "author_email": "jane@company.com",
    "body": "This button needs more padding",
    "resolved": false,
    "created_at": "2026-01-15T10:30:00Z",
    "replies": [
      {
        "id": "uuid",
        "author_name": "John Smith",
        "body": "Agreed, will fix",
        "created_at": "2026-01-15T11:00:00Z"
      }
    ]
  }
]
```

**`POST /api/versions/{id}/comments`**
- Creates a new comment pinned to a location
- Request body:
```json
{
  "page": "index.html",
  "x_percent": 45.2,
  "y_percent": 30.1,
  "body": "This button needs more padding"
}
```
- Author info comes from session (for now, accept `author_name` and `author_email` in the body — Phase 8 replaces with session user)
- Returns the created comment as JSON

**`POST /api/comments/{id}/replies`**
- Adds a reply to a comment
- Request body: `{"body": "reply text"}`
- Author info same as above
- Returns the created reply

**`PATCH /api/comments/{id}/resolve`**
- Toggles the resolved state of a comment
- Returns `{"resolved": true/false}`

### 2. Pin Overlay (JavaScript — `web/static/annotations.js`)

An overlay layer on top of the iframe that handles pin interactions:

**Click to place pin:**
- User clicks on the design area
- Calculate x% and y% relative to the iframe content dimensions
- Show a "new comment" form at the click location
- On submit, POST to create comment API
- After creation, show the pin marker

**Pin markers:**
- Numbered circles (1, 2, 3...) positioned at the comment's x%, y% coordinates
- Pins are absolutely positioned over the iframe
- Resolved pins: dimmed/faded appearance
- Clicking a pin opens its comment thread in the side panel

**Coordinate calculation:**
- Use the iframe's content dimensions (not viewport) for percentage calculation
- `x_percent = (clickX / iframeContentWidth) * 100`
- `y_percent = (clickY / iframeContentHeight) * 100`
- Listen for clicks on an overlay div positioned exactly over the iframe

### 3. Comment Panel (right side or overlay)

When a pin is clicked, show a comment panel:
- Original comment with author name, timestamp, body
- Thread of replies below
- Reply input field
- Resolve/Unresolve button
- Close button

### 4. Comment Filter

Filter controls above the design area or in the sidebar:
- Three options: **All** / **Open** / **Resolved**
- Default: All
- Filtering hides/shows pins and their panel entries
- Filter is client-side only (all comments are already loaded)

### 5. Page-Aware Pins

- Pins are associated with a specific HTML page
- When switching pages (Phase 4's tab navigation), only show pins for the current page
- The `page` field in comments tracks which HTML file the pin belongs to

### 6. Styles

Add to `web/static/style.css`:
- Pin markers: small numbered circles, positioned absolutely
- Pin colors: blue for open, gray for resolved
- Comment panel: card-style with shadow, fixed position or sidebar
- Reply thread: indented, lighter background
- New comment form: simple textarea + submit button
- Filter buttons: toggle-style buttons

### 7. Register Routes

Add to `Handler.RegisterRoutes()`:
- `GET /api/versions/{id}/comments`
- `POST /api/versions/{id}/comments`
- `POST /api/comments/{id}/replies`
- `PATCH /api/comments/{id}/resolve`

## Verification
- Click on the design → pin appears, comment form shows
- Submit a comment → pin is numbered, comment saved
- Click pin → comment thread opens
- Add a reply → appears in thread
- Resolve → pin dims, comment marked resolved
- Filter works: Open shows only unresolved, Resolved shows only resolved
- Switch pages → only that page's pins show
