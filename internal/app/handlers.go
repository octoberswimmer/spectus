package app

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/octoberswimmer/spectus/internal/config"
)

type indexData struct {
	ConfigJSON template.JS
}

type statePayload struct {
	ReturnTo string `json:"return_to"`
	IssuedAt int64  `json:"issued_at"`
}

type Session struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresAt   string `json:"expires_at,omitempty"`
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
	data := indexData{ConfigJSON: cfgJSON}
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
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	payload, err := a.decodeState(state)
	if err != nil {
		http.Error(w, "invalid state", http.StatusUnauthorized)
		return
	}

	conf := a.oauthConfigForRequest(r)
	tok, err := conf.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusUnauthorized)
		return
	}

	scope, _ := tok.Extra("scope").(string)
	session := Session{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		Scope:       scope,
	}
	if !tok.Expiry.IsZero() {
		session.ExpiresAt = tok.Expiry.Format(time.RFC3339)
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

	returnTo := payload.ReturnTo
	if returnTo == "" {
		returnTo = "/"
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	session, err := a.readSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
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
