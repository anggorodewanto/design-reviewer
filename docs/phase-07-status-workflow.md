# Phase 7: Project Status Workflow

## Goal
Add the ability to change project status and display status badges throughout the UI.

## Prerequisites
Phases 1-6 complete — project list, design viewer, versions, and annotations all work.

## What to Build

### 1. Status Update API (`internal/api/projects.go`)

**`PATCH /api/projects/{id}/status`**
- Request body: `{"status": "in_review"}`
- Validates status is one of: `draft`, `in_review`, `approved`, `handed_off`
- Updates the project status and `updated_at`
- Returns: `{"id": "...", "status": "in_review"}`

### 2. Status Change UI (Design Viewer Page)

In the viewer page header (next to project name):
- Show current status as a badge
- A dropdown or button group to change status
- Status options: Draft → In Review → Approved → Handed Off
- Any user can change to any status (no role restrictions)
- On change, PATCH the API and update the badge without full page reload

### 3. Status Badges (Home Page)

The home page (Phase 3) should already show status badges. Verify they work correctly:
- Draft: gray badge
- In Review: blue badge
- Approved: green badge
- Handed Off: purple badge

### 4. JavaScript for Status Change

In `web/static/viewer.js`:
- Status dropdown/buttons in the header
- On click, send PATCH request to update status
- Update the badge text and color on success
- No page reload needed

### 5. Styles

Ensure status badge styles are consistent between home page and viewer page. Add dropdown/button group styles if needed.

## Verification
- New projects start with "Draft" status
- On the viewer page, change status to "In Review" → badge updates
- Go back to home page → status shows "In Review"
- Cycle through all 4 statuses — each displays correct color
- Invalid status values are rejected by the API
