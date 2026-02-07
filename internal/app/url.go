package app

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

func (a *App) baseURLForRequest(r *http.Request) string {
	if strings.TrimSpace(os.Getenv("PUBLIC_URL")) != "" {
		return a.cfg.BaseURL
	}
	if r == nil {
		return a.cfg.BaseURL
	}
	baseURL := requestBaseURL(r)
	if baseURL == "" {
		return a.cfg.BaseURL
	}
	return baseURL
}

func (a *App) oauthConfigForRequest(r *http.Request) *oauth2.Config {
	conf := *a.oauth
	conf.RedirectURL = a.baseURLForRequest(r) + "/auth/github/callback"
	return &conf
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", proto, host)
}

func firstHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
