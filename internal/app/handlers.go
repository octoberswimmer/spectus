package app

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/octoberswimmer/spectus/internal/config"
)

type indexData struct {
	ConfigJSON    template.JS
	StaticVersion string
}

type statePayload struct {
	ReturnTo string `json:"return_to"`
	IssuedAt int64  `json:"issued_at"`
}

type Session struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	RefreshTokenExpiresAt string `json:"refresh_token_expires_at,omitempty"`
	SelectedRepo          string `json:"selected_repo,omitempty"`
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	cfgJSON, err := a.clientConfigJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := indexData{
		ConfigJSON:    cfgJSON,
		StaticVersion: strconv.FormatInt(time.Now().Unix(), 10),
	}
	if err := a.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := a.encodeState(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conf := a.oauthConfigForRequest(r)
	url := conf.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (a *App) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Determine return URL from state
	// State can be either:
	// 1. An encoded state payload (from normal OAuth flow via /login)
	// 2. A raw URL (from GitHub App installation flow)
	returnTo := "/"
	if state != "" {
		if payload, err := a.decodeState(state); err == nil {
			returnTo = payload.ReturnTo
		} else if strings.HasPrefix(state, "http://") || strings.HasPrefix(state, "https://") {
			// State is a raw URL from the install flow
			returnTo = state
		}
	}

	conf := a.oauthConfigForRequest(r)
	tok, err := conf.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusUnauthorized)
		return
	}

	scope, _ := tok.Extra("scope").(string)
	session := Session{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		Scope:        scope,
		RefreshToken: tok.RefreshToken,
	}
	if !tok.Expiry.IsZero() {
		session.ExpiresAt = tok.Expiry.Format(time.RFC3339)
	}
	if refreshExpiresIn, ok := tok.Extra("refresh_token_expires_in").(float64); ok && refreshExpiresIn > 0 {
		refreshExpiry := time.Now().Add(time.Duration(refreshExpiresIn) * time.Second)
		session.RefreshTokenExpiresAt = refreshExpiry.Format(time.RFC3339)
	}

	encoded, err := a.cookies.Encode(a.cookieName, session)
	if err != nil {
		http.Error(w, "failed to encode session", http.StatusInternalServerError)
		return
	}

	cookie := &http.Cookie{
		Name:     a.cookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   config.IsSecureURL(a.baseURLForRequest(r)),
	}
	http.SetCookie(w, cookie)

	if returnTo == "" {
		returnTo = "/"
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		session, err := a.readSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	case http.MethodPost:
		session, err := a.readSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload struct {
			SelectedRepo string `json:"selected_repo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		session.SelectedRepo = strings.TrimSpace(payload.SelectedRepo)
		encoded, err := a.cookies.Encode(a.cookieName, session)
		if err != nil {
			http.Error(w, "failed to encode session", http.StatusInternalServerError)
			return
		}
		cookie := &http.Cookie{
			Name:     a.cookieName,
			Value:    encoded,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   config.IsSecureURL(a.baseURLForRequest(r)),
		}
		http.SetCookie(w, cookie)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   config.IsSecureURL(a.baseURLForRequest(r)),
	}
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"logged out"}`))
}

func (a *App) encodeState(r *http.Request) (string, error) {
	returnTo := r.URL.Query().Get("next")
	if returnTo == "" {
		returnTo = "/"
	}
	if !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		returnTo = "/"
	}
	payload := statePayload{ReturnTo: returnTo, IssuedAt: time.Now().Unix()}
	encoded, err := a.cookies.Encode("state", payload)
	if err != nil {
		return "", err
	}
	return encoded, nil
}

func (a *App) decodeState(value string) (statePayload, error) {
	var payload statePayload
	if err := a.cookies.Decode("state", value, &payload); err != nil {
		return statePayload{}, err
	}
	return payload, nil
}

func (a *App) readSession(r *http.Request) (Session, error) {
	cookie, err := r.Cookie(a.cookieName)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := a.cookies.Decode(a.cookieName, cookie.Value, &session); err != nil {
		return Session{}, err
	}
	if session.AccessToken == "" {
		return Session{}, errors.New("missing access token")
	}
	return session, nil
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	session, err := a.readSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if session.RefreshToken == "" {
		http.Error(w, "no refresh token available", http.StatusBadRequest)
		return
	}

	newSession, err := a.refreshToken(r.Context(), session.RefreshToken)
	if err != nil {
		http.Error(w, "failed to refresh token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Preserve selected repo from old session
	newSession.SelectedRepo = session.SelectedRepo

	encoded, err := a.cookies.Encode(a.cookieName, newSession)
	if err != nil {
		http.Error(w, "failed to encode session", http.StatusInternalServerError)
		return
	}

	cookie := &http.Cookie{
		Name:     a.cookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   config.IsSecureURL(a.baseURLForRequest(r)),
	}
	http.SetCookie(w, cookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newSession)
}

func (a *App) refreshToken(ctx context.Context, refreshToken string) (Session, error) {
	data := url.Values{}
	data.Set("client_id", a.oauth.ClientID)
	data.Set("client_secret", a.oauth.ClientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	tokenURL := a.tokenURL
	if tokenURL == "" {
		tokenURL = "https://github.com/login/oauth/access_token"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Session{}, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken           string `json:"access_token"`
		TokenType             string `json:"token_type"`
		Scope                 string `json:"scope"`
		ExpiresIn             int    `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
		Error                 string `json:"error"`
		ErrorDescription      string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Session{}, err
	}

	if result.Error != "" {
		return Session{}, errors.New(result.ErrorDescription)
	}

	session := Session{
		AccessToken:  result.AccessToken,
		TokenType:    result.TokenType,
		Scope:        result.Scope,
		RefreshToken: result.RefreshToken,
	}

	if result.ExpiresIn > 0 {
		session.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	if result.RefreshTokenExpiresIn > 0 {
		session.RefreshTokenExpiresAt = time.Now().Add(time.Duration(result.RefreshTokenExpiresIn) * time.Second).Format(time.RFC3339)
	}

	return session, nil
}
