package app

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/octoberswimmer/spectus/internal/config"
)

type App struct {
	cfg        config.Config
	tmpl       *template.Template
	oauth      *oauth2.Config
	cookies    *securecookie.SecureCookie
	cookieName string
	sseHub     *SSEHub
}

type ClientConfig struct {
	DefaultRepo   string `json:"default_repo"`
	KanbanPath    string `json:"kanban_path"`
	ArchivePath   string `json:"archive_path"`
	AppInstallURL string `json:"app_install_url"`
}

func New(cfg config.Config, tmpl *template.Template) *App {
	cookies := securecookie.New(cfg.HashKey, cfg.BlockKey)
	oauth := &oauth2.Config{
		ClientID:     cfg.OAuthClientID,
		ClientSecret: cfg.OAuthClientSecret,
		Scopes:       nil,
		RedirectURL:  cfg.BaseURL + "/auth/github/callback",
		Endpoint:     github.Endpoint,
	}
	return &App{
		cfg:        cfg,
		tmpl:       tmpl,
		oauth:      oauth,
		cookies:    cookies,
		cookieName: cfg.SessionCookieName,
		sseHub:     NewSSEHub(),
	}
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/login", a.handleLogin)
	mux.HandleFunc("/auth/github/callback", a.handleCallback)
	mux.HandleFunc("/session", a.handleSession)
	mux.HandleFunc("/logout", a.handleLogout)
	mux.HandleFunc("/webhook", a.handleWebhook)
	mux.HandleFunc("/events", a.handleSSE)
}

func (a *App) clientConfigJSON() (template.JS, error) {
	cfg := ClientConfig{
		DefaultRepo:   a.cfg.DefaultRepo,
		KanbanPath:    a.cfg.KanbanPath,
		ArchivePath:   a.cfg.ArchivePath,
		AppInstallURL: a.cfg.AppInstallURL,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "{}", err
	}
	return template.JS(string(data)), nil
}
