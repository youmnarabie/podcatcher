// cmd/server/main.go
package main

import (
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rs/cors"
	"github.com/youmnarabie/poo/internal/api"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/poller"
	"github.com/youmnarabie/poo/internal/store"
	"github.com/youmnarabie/poo/web"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	if err := store.RunMigrations(dbURL, migrationsPath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()

	ing := ingester.New(s)
	srv := api.New(s, ing)

	poll := poller.New(s, ing, time.Hour)
	go poll.Run(ctx)

	webFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatalf("web fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try static file; fall back to index.html for SPA routing
		f, err := webFS.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		idx, err := webFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer idx.Close()
		http.ServeContent(w, r, "index.html", time.Time{}, idx.(io.ReadSeeker))
	})

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type"},
	})

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, c.Handler(mux)); err != nil {
		log.Fatal(err)
	}
}
