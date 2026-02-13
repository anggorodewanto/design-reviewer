package seed

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/ab/design-reviewer/internal/db"
)

//go:embed landing/*
var landingFiles embed.FS

func Run(database *db.DB, uploadsDir string) {
	projects, err := database.ListProjects()
	if err != nil || len(projects) > 0 {
		return
	}
	p, err := database.CreateProject("Design Reviewer — Landing Page", "")
	if err != nil {
		log.Printf("seed: create project: %v", err)
		return
	}
	v, err := database.CreateVersion(p.ID, filepath.Join(uploadsDir, "seed"))
	if err != nil {
		log.Printf("seed: create version: %v", err)
		return
	}
	dir := filepath.Join(uploadsDir, v.ID)
	os.MkdirAll(dir, 0o755)
	entries, _ := landingFiles.ReadDir("landing")
	for _, e := range entries {
		data, _ := landingFiles.ReadFile("landing/" + e.Name())
		os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644)
	}
	log.Printf("seed: created default project %q", p.Name)
}
