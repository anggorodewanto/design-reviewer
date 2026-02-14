package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Project struct {
	ID         string
	Name       string
	OwnerEmail *string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ProjectInvite struct {
	ID        string
	ProjectID string
	Token     string
	CreatedBy string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

type ProjectMember struct {
	ProjectID string
	UserEmail string
	AddedAt   time.Time
}

type Version struct {
	ID          string
	ProjectID   string
	VersionNum  int
	StoragePath string
	CreatedAt   time.Time
}

type Comment struct {
	ID          string
	VersionID   string
	Page        string
	XPercent    float64
	YPercent    float64
	AuthorName  string
	AuthorEmail string
	Body        string
	Resolved    bool
	CreatedAt   time.Time
}

type Reply struct {
	ID          string
	CommentID   string
	AuthorName  string
	AuthorEmail string
	Body        string
	CreatedAt   time.Time
}

type DB struct {
	*sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    owner_email TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS versions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    version_num INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL REFERENCES versions(id),
    page TEXT NOT NULL,
    x_percent REAL NOT NULL,
    y_percent REAL NOT NULL,
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    body TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS replies (
    id TEXT PRIMARY KEY,
    comment_id TEXT NOT NULL REFERENCES comments(id),
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    user_name TEXT NOT NULL,
    user_email TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL DEFAULT (datetime('now', '+90 days'))
);

CREATE TABLE IF NOT EXISTS project_invites (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    token TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

CREATE TABLE IF NOT EXISTS project_members (
    project_id TEXT NOT NULL REFERENCES projects(id),
    user_email TEXT NOT NULL,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, user_email)
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_name TEXT NOT NULL,
    user_email TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func New(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		return nil, err
	}
	// Migration: add expires_at to tokens if missing
	sqlDB.Exec(`ALTER TABLE tokens ADD COLUMN expires_at DATETIME NOT NULL DEFAULT (datetime('now', '+90 days'))`)
	return &DB{sqlDB}, nil
}

// --- Projects ---

func (d *DB) CreateProject(name, ownerEmail string) (*Project, error) {
	p := &Project{
		ID:     uuid.NewString(),
		Name:   name,
		Status: "draft",
	}
	var owner *string
	if ownerEmail != "" {
		owner = &ownerEmail
	}
	p.OwnerEmail = owner
	err := d.QueryRow(
		`INSERT INTO projects (id, name, owner_email, status) VALUES (?, ?, ?, ?) RETURNING created_at, updated_at`,
		p.ID, p.Name, owner, p.Status,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) GetProject(id string) (*Project, error) {
	p := &Project{}
	err := d.QueryRow(`SELECT id, name, owner_email, status, created_at, updated_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.OwnerEmail, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) GetProjectByName(name string) (*Project, error) {
	p := &Project{}
	err := d.QueryRow(`SELECT id, name, owner_email, status, created_at, updated_at FROM projects WHERE name = ?`, name).
		Scan(&p.ID, &p.Name, &p.OwnerEmail, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) ListProjects() ([]Project, error) {
	rows, err := d.Query(`SELECT id, name, owner_email, status, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.OwnerEmail, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

type ProjectWithVersionCount struct {
	ID           string
	Name         string
	Status       string
	VersionCount int
	UpdatedAt    time.Time
}

func (d *DB) ListProjectsWithVersionCount() ([]ProjectWithVersionCount, error) {
	rows, err := d.Query(`
		SELECT p.id, p.name, p.status, COUNT(v.id) AS version_count, p.updated_at
		FROM projects p
		LEFT JOIN versions v ON v.project_id = p.id
		GROUP BY p.id
		ORDER BY p.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []ProjectWithVersionCount
	for rows.Next() {
		var p ProjectWithVersionCount
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.VersionCount, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

var validStatuses = map[string]bool{
	"draft": true, "in_review": true, "approved": true, "handed_off": true,
}

func (d *DB) UpdateProjectStatus(id, status string) error {
	if !validStatuses[status] {
		return fmt.Errorf("invalid status %q: must be one of draft, in_review, approved, handed_off", status)
	}
	res, err := d.Exec(`UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- Versions ---

func (d *DB) CreateVersion(projectID, storagePath string) (*Version, error) {
	v := &Version{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		StoragePath: storagePath,
	}
	err := d.QueryRow(
		`INSERT INTO versions (id, project_id, version_num, storage_path)
		 VALUES (?, ?, COALESCE((SELECT MAX(version_num) FROM versions WHERE project_id = ?), 0) + 1, ?)
		 RETURNING version_num, created_at`,
		v.ID, v.ProjectID, v.ProjectID, v.StoragePath,
	).Scan(&v.VersionNum, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *DB) GetVersion(id string) (*Version, error) {
	v := &Version{}
	err := d.QueryRow(`SELECT id, project_id, version_num, storage_path, created_at FROM versions WHERE id = ?`, id).
		Scan(&v.ID, &v.ProjectID, &v.VersionNum, &v.StoragePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *DB) ListVersions(projectID string) ([]Version, error) {
	rows, err := d.Query(`SELECT id, project_id, version_num, storage_path, created_at FROM versions WHERE project_id = ? ORDER BY version_num DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.VersionNum, &v.StoragePath, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (d *DB) GetLatestVersion(projectID string) (*Version, error) {
	v := &Version{}
	err := d.QueryRow(
		`SELECT id, project_id, version_num, storage_path, created_at FROM versions WHERE project_id = ? ORDER BY version_num DESC LIMIT 1`,
		projectID,
	).Scan(&v.ID, &v.ProjectID, &v.VersionNum, &v.StoragePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// --- Comments ---

func (d *DB) CreateComment(versionID, page string, xPercent, yPercent float64, authorName, authorEmail, body string) (*Comment, error) {
	c := &Comment{
		ID:          uuid.NewString(),
		VersionID:   versionID,
		Page:        page,
		XPercent:    xPercent,
		YPercent:    yPercent,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		Body:        body,
	}
	err := d.QueryRow(
		`INSERT INTO comments (id, version_id, page, x_percent, y_percent, author_name, author_email, body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING resolved, created_at`,
		c.ID, c.VersionID, c.Page, c.XPercent, c.YPercent, c.AuthorName, c.AuthorEmail, c.Body,
	).Scan(&c.Resolved, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d *DB) GetCommentsForVersion(versionID string) ([]Comment, error) {
	rows, err := d.Query(
		`SELECT id, version_id, page, x_percent, y_percent, author_name, author_email, body, resolved, created_at
		 FROM comments WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.VersionID, &c.Page, &c.XPercent, &c.YPercent, &c.AuthorName, &c.AuthorEmail, &c.Body, &c.Resolved, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (d *DB) GetUnresolvedCommentsUpTo(versionID string) ([]Comment, error) {
	rows, err := d.Query(
		`SELECT c.id, c.version_id, c.page, c.x_percent, c.y_percent, c.author_name, c.author_email, c.body, c.resolved, c.created_at
		 FROM comments c
		 JOIN versions v ON c.version_id = v.id
		 WHERE c.resolved = 0
		   AND v.project_id = (SELECT project_id FROM versions WHERE id = ?)
		   AND v.version_num <= (SELECT version_num FROM versions WHERE id = ?)`,
		versionID, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.VersionID, &c.Page, &c.XPercent, &c.YPercent, &c.AuthorName, &c.AuthorEmail, &c.Body, &c.Resolved, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (d *DB) GetComment(id string) (*Comment, error) {
	c := &Comment{}
	err := d.QueryRow(`SELECT id, version_id, page, x_percent, y_percent, author_name, author_email, body, resolved, created_at FROM comments WHERE id = ?`, id).
		Scan(&c.ID, &c.VersionID, &c.Page, &c.XPercent, &c.YPercent, &c.AuthorName, &c.AuthorEmail, &c.Body, &c.Resolved, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d *DB) MoveComment(id string, x, y float64) error {
	_, err := d.Exec("UPDATE comments SET x_percent=?, y_percent=? WHERE id=?", x, y, id)
	return err
}

func (d *DB) ToggleResolve(commentID string) (bool, error) {
	var resolved bool
	err := d.QueryRow(`UPDATE comments SET resolved = NOT resolved WHERE id = ? RETURNING resolved`, commentID).Scan(&resolved)
	if err != nil {
		return false, err
	}
	return resolved, nil
}

// --- Replies ---

func (d *DB) CreateReply(commentID, authorName, authorEmail, body string) (*Reply, error) {
	r := &Reply{
		ID:          uuid.NewString(),
		CommentID:   commentID,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		Body:        body,
	}
	err := d.QueryRow(
		`INSERT INTO replies (id, comment_id, author_name, author_email, body)
		 VALUES (?, ?, ?, ?, ?) RETURNING created_at`,
		r.ID, r.CommentID, r.AuthorName, r.AuthorEmail, r.Body,
	).Scan(&r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (d *DB) GetReplies(commentID string) ([]Reply, error) {
	rows, err := d.Query(
		`SELECT id, comment_id, author_name, author_email, body, created_at
		 FROM replies WHERE comment_id = ? ORDER BY created_at ASC`, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var replies []Reply
	for rows.Next() {
		var r Reply
		if err := rows.Scan(&r.ID, &r.CommentID, &r.AuthorName, &r.AuthorEmail, &r.Body, &r.CreatedAt); err != nil {
			return nil, err
		}
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

// --- Tokens ---

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (d *DB) CreateToken(token, userName, userEmail string) error {
	_, err := d.Exec(`INSERT INTO tokens (token, user_name, user_email, expires_at) VALUES (?, ?, ?, datetime('now', '+90 days'))`, hashToken(token), userName, userEmail)
	return err
}

func (d *DB) GetUserByToken(token string) (name, email string, err error) {
	err = d.QueryRow(`SELECT user_name, user_email FROM tokens WHERE token = ? AND expires_at > CURRENT_TIMESTAMP`, hashToken(token)).Scan(&name, &email)
	return
}

// --- Sharing ---

func (d *DB) ListProjectsWithVersionCountForUser(email string) ([]ProjectWithVersionCount, error) {
	rows, err := d.Query(`
		SELECT p.id, p.name, p.status, COUNT(v.id) AS version_count, p.updated_at
		FROM projects p
		LEFT JOIN versions v ON v.project_id = p.id
		WHERE p.owner_email IS NULL
		   OR p.owner_email = ?
		   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = ?)
		GROUP BY p.id
		ORDER BY p.updated_at DESC`, email, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []ProjectWithVersionCount
	for rows.Next() {
		var p ProjectWithVersionCount
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.VersionCount, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (d *DB) CanAccessProject(projectID, email string) (bool, error) {
	var count int
	err := d.QueryRow(`
		SELECT COUNT(*) FROM projects p
		WHERE p.id = ?
		  AND (p.owner_email IS NULL OR p.owner_email = ?
		       OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = ?))`,
		projectID, email, email).Scan(&count)
	return count > 0, err
}

func (d *DB) GetProjectOwner(projectID string) (string, error) {
	var owner sql.NullString
	err := d.QueryRow(`SELECT owner_email FROM projects WHERE id = ?`, projectID).Scan(&owner)
	if err != nil {
		return "", err
	}
	return owner.String, nil
}

func (d *DB) CreateInvite(projectID, createdBy string) (*ProjectInvite, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	inv := &ProjectInvite{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Token:     hex.EncodeToString(b),
		CreatedBy: createdBy,
	}
	err := d.QueryRow(
		`INSERT INTO project_invites (id, project_id, token, created_by) VALUES (?, ?, ?, ?) RETURNING created_at`,
		inv.ID, inv.ProjectID, inv.Token, inv.CreatedBy,
	).Scan(&inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (d *DB) GetInviteByToken(token string) (*ProjectInvite, error) {
	inv := &ProjectInvite{}
	err := d.QueryRow(
		`SELECT id, project_id, token, created_by, created_at, expires_at FROM project_invites WHERE token = ?`, token,
	).Scan(&inv.ID, &inv.ProjectID, &inv.Token, &inv.CreatedBy, &inv.CreatedAt, &inv.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now()) {
		return nil, sql.ErrNoRows
	}
	return inv, nil
}

func (d *DB) DeleteInvite(id string) error {
	_, err := d.Exec(`DELETE FROM project_invites WHERE id = ?`, id)
	return err
}

func (d *DB) AddMember(projectID, email string) error {
	_, err := d.Exec(
		`INSERT OR IGNORE INTO project_members (project_id, user_email) VALUES (?, ?)`,
		projectID, email)
	return err
}

func (d *DB) ListMembers(projectID string) ([]ProjectMember, error) {
	rows, err := d.Query(
		`SELECT project_id, user_email, added_at FROM project_members WHERE project_id = ? ORDER BY added_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserEmail, &m.AddedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (d *DB) RemoveMember(projectID, email string) error {
	_, err := d.Exec(`DELETE FROM project_members WHERE project_id = ? AND user_email = ?`, projectID, email)
	return err
}

// --- Sessions ---

func (d *DB) CreateSession(id, userName, userEmail string) error {
	_, err := d.Exec(`INSERT INTO sessions (id, user_name, user_email) VALUES (?, ?, ?)`, id, userName, userEmail)
	return err
}

func (d *DB) GetSession(id string) (string, string, error) {
	var name, email string
	err := d.QueryRow(`SELECT user_name, user_email FROM sessions WHERE id = ?`, id).Scan(&name, &email)
	return name, email, err
}

func (d *DB) DeleteSession(id string) error {
	_, err := d.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}
