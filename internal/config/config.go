package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/securecookie"
)

type Config struct {
	Addr              string
	BaseURL           string
	OAuthClientID     string
	OAuthClientSecret string
	OAuthScopes       []string
	AppInstallURL     string
	HashKey           []byte
	BlockKey          []byte
	DefaultRepo       string
	KanbanPath        string
	ArchivePath       string
	SessionCookieName string
	WebhookSecret     string
}

func Load() Config {
	addr := envOr("ADDR", "")
	if addr == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			addr = ":" + port
		} else {
			addr = ":8080"
		}
	}
	baseURL := strings.TrimSpace(os.Getenv("PUBLIC_URL"))
	if baseURL == "" {
		baseURL = defaultBaseURL(addr)
	}

	cfg := Config{
		Addr:              addr,
		BaseURL:           baseURL,
		OAuthClientID:     strings.TrimSpace(os.Getenv("GITHUB_APP_CLIENT_ID")),
		OAuthClientSecret: strings.TrimSpace(os.Getenv("GITHUB_APP_CLIENT_SECRET")),
		OAuthScopes:       envList("GITHUB_SCOPES", nil),
		AppInstallURL:     strings.TrimSpace(os.Getenv("GITHUB_APP_INSTALL_URL")),
		DefaultRepo:       strings.TrimSpace(os.Getenv("KANBAN_REPO")),
		KanbanPath:        envOr("KANBAN_PATH", "kanban.md"),
		ArchivePath:       envOr("ARCHIVE_PATH", "archive.md"),
		SessionCookieName: envOr("SESSION_COOKIE", "kanban_session"),
		WebhookSecret:     strings.TrimSpace(os.Getenv("GITHUB_WEBHOOK_SECRET")),
	}

	cfg.HashKey = []byte(strings.TrimSpace(os.Getenv("HASH_KEY")))
	cfg.BlockKey = []byte(strings.TrimSpace(os.Getenv("BLOCK_KEY")))

	if len(cfg.HashKey) == 0 {
		cfg.HashKey = securecookie.GenerateRandomKey(32)
	}
	if len(cfg.BlockKey) == 0 {
		cfg.BlockKey = securecookie.GenerateRandomKey(32)
	}

	return cfg
}

func defaultBaseURL(addr string) string {
	host := strings.TrimSpace(addr)
	if host == "" {
		return "http://localhost:8080"
	}
	if strings.HasPrefix(host, ":") {
		return fmt.Sprintf("http://localhost%s", host)
	}
	if strings.Contains(host, ":") {
		return fmt.Sprintf("http://%s", host)
	}
	if _, err := net.LookupHost(host); err == nil {
		return fmt.Sprintf("http://%s", host)
	}
	return fmt.Sprintf("http://%s", host)
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		if fallback == nil {
			return nil
		}
		return append([]string{}, fallback...)
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func IsSecureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "https"
}
