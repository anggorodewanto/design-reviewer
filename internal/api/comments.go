package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ab/design-reviewer/internal/auth"
)

type commentJSON struct {
	ID          string      `json:"id"`
	VersionID   string      `json:"version_id"`
	Page        string      `json:"page"`
	XPercent    float64     `json:"x_percent"`
	YPercent    float64     `json:"y_percent"`
	AuthorName  string      `json:"author_name"`
	AuthorEmail string      `json:"author_email"`
	Body        string      `json:"body"`
	Resolved    bool        `json:"resolved"`
	CreatedAt   string      `json:"created_at"`
	Replies     []replyJSON `json:"replies"`
}

type replyJSON struct {
	ID         string `json:"id"`
	AuthorName string `json:"author_name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

func (h *Handler) handleGetComments(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("id")

	comments, err := h.DB.GetUnresolvedCommentsUpTo(versionID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Also get resolved comments for this specific version
	allForVersion, err := h.DB.GetCommentsForVersion(versionID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Merge: unresolved from all versions up to this one + resolved from this version
	seen := map[string]bool{}
	for _, c := range comments {
		seen[c.ID] = true
	}
	for _, c := range allForVersion {
		if c.Resolved && !seen[c.ID] {
			comments = append(comments, c)
			seen[c.ID] = true
		}
	}

	out := make([]commentJSON, 0, len(comments))
	for _, c := range comments {
		replies, err := h.DB.GetReplies(c.ID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		rj := make([]replyJSON, len(replies))
		for i, r := range replies {
			rj[i] = replyJSON{
				ID:         r.ID,
				AuthorName: r.AuthorName,
				Body:       r.Body,
				CreatedAt:  r.CreatedAt.Format(time.RFC3339),
			}
		}
		out = append(out, commentJSON{
			ID:          c.ID,
			VersionID:   c.VersionID,
			Page:        c.Page,
			XPercent:    c.XPercent,
			YPercent:    c.YPercent,
			AuthorName:  c.AuthorName,
			AuthorEmail: c.AuthorEmail,
			Body:        c.Body,
			Resolved:    c.Resolved,
			CreatedAt:   c.CreatedAt.Format(time.RFC3339),
			Replies:     rj,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		Page        string  `json:"page"`
		XPercent    float64 `json:"x_percent"`
		YPercent    float64 `json:"y_percent"`
		AuthorName  string  `json:"author_name"`
		AuthorEmail string  `json:"author_email"`
		Body        string  `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isMaxBytesError(err) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Body == "" || req.Page == "" {
		http.Error(w, "body and page are required", http.StatusBadRequest)
		return
	}

	// Use auth context if available, fall back to request body
	if name, email := auth.GetUserFromContext(r.Context()); name != "" {
		req.AuthorName = name
		req.AuthorEmail = email
	}

	c, err := h.DB.CreateComment(versionID, req.Page, req.XPercent, req.YPercent, req.AuthorName, req.AuthorEmail, req.Body)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(commentJSON{
		ID:          c.ID,
		VersionID:   c.VersionID,
		Page:        c.Page,
		XPercent:    c.XPercent,
		YPercent:    c.YPercent,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
		Resolved:    c.Resolved,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
		Replies:     []replyJSON{},
	})
}

func (h *Handler) handleCreateReply(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
		Body        string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isMaxBytesError(err) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	// Use auth context if available, fall back to request body
	if name, email := auth.GetUserFromContext(r.Context()); name != "" {
		req.AuthorName = name
		req.AuthorEmail = email
	}

	reply, err := h.DB.CreateReply(commentID, req.AuthorName, req.AuthorEmail, req.Body)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(replyJSON{
		ID:         reply.ID,
		AuthorName: reply.AuthorName,
		Body:       reply.Body,
		CreatedAt:  reply.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) handleMoveComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		XPercent float64 `json:"x_percent"`
		YPercent float64 `json:"y_percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isMaxBytesError(err) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.XPercent < 0 || req.XPercent > 100 || req.YPercent < 0 || req.YPercent > 100 {
		http.Error(w, "x_percent and y_percent must be between 0 and 100", http.StatusBadRequest)
		return
	}
	if err := h.DB.MoveComment(commentID, req.XPercent, req.YPercent); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *Handler) handleToggleResolve(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("id")

	resolved, err := h.DB.ToggleResolve(commentID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"resolved": resolved})
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
