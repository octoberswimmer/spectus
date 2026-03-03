package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/securecookie"
	"github.com/octoberswimmer/spectus/internal/config"
	"golang.org/x/oauth2"
)

func TestHandleRefresh_no_session_returns_unauthorized(t *testing.T) {
	app := newTestAppWithCookies()
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rec := httptest.NewRecorder()

	app.handleRefresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestHandleRefresh_no_refresh_token_returns_bad_request(t *testing.T) {
	app := newTestAppWithCookies()

	// Create a session without a refresh token
	session := Session{
		AccessToken: "test-access-token",
		TokenType:   "bearer",
	}
	cookie := app.createSessionCookie(session)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	app.handleRefresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleRefresh_with_refresh_token_calls_github(t *testing.T) {
	// Set up a mock GitHub OAuth server
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login/oauth/access_token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("refresh_token") != "test-refresh-token" {
			t.Errorf("expected refresh_token=test-refresh-token, got %s", r.Form.Get("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":             "new-access-token",
			"token_type":               "bearer",
			"scope":                    "repo",
			"expires_in":               28800,
			"refresh_token":            "new-refresh-token",
			"refresh_token_expires_in": 15811200,
		})
	}))
	defer mockGitHub.Close()

	app := newTestAppWithCookies()
	app.oauth = &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	app.tokenURL = mockGitHub.URL + "/login/oauth/access_token"

	// Create a session with a refresh token
	session := Session{
		AccessToken:  "old-access-token",
		TokenType:    "bearer",
		RefreshToken: "test-refresh-token",
	}
	cookie := app.createSessionCookie(session)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	app.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response Session
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.AccessToken != "new-access-token" {
		t.Errorf("expected new-access-token, got %s", response.AccessToken)
	}
	if response.RefreshToken != "new-refresh-token" {
		t.Errorf("expected new-refresh-token, got %s", response.RefreshToken)
	}
}

func TestHandleRefresh_preserves_selected_repo(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"token_type":    "bearer",
			"refresh_token": "new-refresh-token",
		})
	}))
	defer mockGitHub.Close()

	app := newTestAppWithCookies()
	app.oauth = &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	app.tokenURL = mockGitHub.URL + "/login/oauth/access_token"

	// Create a session with a refresh token and selected repo
	session := Session{
		AccessToken:  "old-access-token",
		TokenType:    "bearer",
		RefreshToken: "test-refresh-token",
		SelectedRepo: "owner/repo",
	}
	cookie := app.createSessionCookie(session)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	app.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response Session
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.SelectedRepo != "owner/repo" {
		t.Errorf("expected selected repo to be preserved, got %s", response.SelectedRepo)
	}
}

func TestHandleRefresh_github_error_returns_unauthorized(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "bad_refresh_token",
			"error_description": "The refresh token is invalid",
		})
	}))
	defer mockGitHub.Close()

	app := newTestAppWithCookies()
	app.oauth = &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	app.tokenURL = mockGitHub.URL + "/login/oauth/access_token"

	session := Session{
		AccessToken:  "old-access-token",
		TokenType:    "bearer",
		RefreshToken: "invalid-refresh-token",
	}
	cookie := app.createSessionCookie(session)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	app.handleRefresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func newTestAppWithCookies() *App {
	hashKey := []byte("12345678901234567890123456789012")
	blockKey := []byte("12345678901234567890123456789012")
	return &App{
		cfg:        config.Config{},
		sseHub:     NewSSEHub(),
		cookies:    securecookie.New(hashKey, blockKey),
		cookieName: "test_session",
	}
}

func (a *App) createSessionCookie(session Session) *http.Cookie {
	encoded, _ := a.cookies.Encode(a.cookieName, session)
	return &http.Cookie{
		Name:  a.cookieName,
		Value: encoded,
	}
}
