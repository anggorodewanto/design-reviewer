package api

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/ab/design-reviewer/internal/auth"
)

func (h *Handler) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	_, email := auth.GetUserFromContext(r.Context())

	inv, err := h.DB.CreateInvite(projectID, email)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := ""
	if h.Auth != nil {
		baseURL = h.Auth.BaseURL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":         inv.ID,
		"invite_url": baseURL + "/invite/" + inv.Token,
	})
}

func (h *Handler) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	inviteID := r.PathValue("inviteID")
	if err := h.DB.DeleteInvite(inviteID); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	members, err := h.DB.ListMembers(projectID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	type memberJSON struct {
		Email   string `json:"email"`
		AddedAt string `json:"added_at"`
	}
	out := make([]memberJSON, len(members))
	for i, m := range members {
		out[i] = memberJSON{Email: m.UserEmail, AddedAt: m.AddedAt.Format(time.RFC3339)}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	email := r.PathValue("email")

	// Cannot remove the owner
	owner, err := h.DB.GetProjectOwner(projectID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if email == owner {
		http.Error(w, "cannot remove owner", http.StatusBadRequest)
		return
	}

	if err := h.DB.RemoveMember(projectID, email); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	inv, err := h.DB.GetInviteByToken(token)
	if err == sql.ErrNoRows {
		tmpl, tErr := template.ParseFiles(
			filepath.Join(h.TemplatesDir, "layout.html"),
			filepath.Join(h.TemplatesDir, "invite.html"),
		)
		if tErr != nil {
			http.Error(w, "invalid or expired invite", http.StatusNotFound)
			return
		}
		name, _ := auth.GetUserFromContext(r.Context())
		tmpl.Execute(w, struct {
			Error    string
			UserName string
		}{"This invite link is invalid or has expired.", name})
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	_, email := auth.GetUserFromContext(r.Context())
	if err := h.DB.AddMember(inv.ProjectID, email); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/projects/"+inv.ProjectID, http.StatusFound)
}
