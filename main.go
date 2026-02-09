package main

import (
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/octoberswimmer/spectus/internal/app"
	"github.com/octoberswimmer/spectus/internal/config"
)

//go:embed templates/* static/*
var content embed.FS

func main() {
	cfg := config.Load()

	tmpl := template.Must(template.ParseFS(content, "templates/*.html"))
	webApp := app.New(cfg, tmpl)

	mux := http.NewServeMux()

	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	_ = mime.AddExtensionType(".wasm", "application/wasm")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	webApp.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.WithLogging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("spectus listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
