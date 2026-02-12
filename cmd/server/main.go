package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/ab/design-reviewer/internal/api"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
)

func main() {
	port := flag.Int("port", 8080, "server port")
	dbPath := flag.String("db", "./data/design-reviewer.db", "SQLite database path")
	uploads := flag.String("uploads", "./data/uploads", "upload directory")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	store := storage.New(*uploads)

	h := &api.Handler{DB: database, Storage: store}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("server running on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
