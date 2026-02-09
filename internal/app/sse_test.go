package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/octoberswimmer/spectus/internal/config"
)

func TestSSEHub_subscribe_creates_channel_for_repo(t *testing.T) {
	hub := NewSSEHub()

	ch := hub.Subscribe("owner/repo")

	if ch == nil {
		t.Fatal("expected channel, got nil")
	}
	if len(hub.clients) != 1 {
		t.Errorf("expected 1 repo in clients, got %d", len(hub.clients))
	}
	if len(hub.clients["owner/repo"]) != 1 {
		t.Errorf("expected 1 client for repo, got %d", len(hub.clients["owner/repo"]))
	}
}

func TestSSEHub_subscribe_multiple_clients_same_repo(t *testing.T) {
	hub := NewSSEHub()

	ch1 := hub.Subscribe("owner/repo")
	ch2 := hub.Subscribe("owner/repo")

	if ch1 == ch2 {
		t.Error("expected different channels for different subscriptions")
	}
	if len(hub.clients["owner/repo"]) != 2 {
		t.Errorf("expected 2 clients for repo, got %d", len(hub.clients["owner/repo"]))
	}
}

func TestSSEHub_unsubscribe_removes_client(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe("owner/repo")

	hub.Unsubscribe("owner/repo", ch)

	if len(hub.clients) != 0 {
		t.Errorf("expected 0 repos after unsubscribe, got %d", len(hub.clients))
	}
}

func TestSSEHub_unsubscribe_closes_channel(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe("owner/repo")

	hub.Unsubscribe("owner/repo", ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	default:
		t.Error("expected channel to be closed, but it blocked")
	}
}

func TestSSEHub_unsubscribe_keeps_other_clients(t *testing.T) {
	hub := NewSSEHub()
	ch1 := hub.Subscribe("owner/repo")
	ch2 := hub.Subscribe("owner/repo")

	hub.Unsubscribe("owner/repo", ch1)

	if len(hub.clients["owner/repo"]) != 1 {
		t.Errorf("expected 1 client remaining, got %d", len(hub.clients["owner/repo"]))
	}
	if _, exists := hub.clients["owner/repo"][ch2]; !exists {
		t.Error("expected ch2 to still be subscribed")
	}
}

func TestSSEHub_notify_sends_to_all_subscribers(t *testing.T) {
	hub := NewSSEHub()
	ch1 := hub.Subscribe("owner/repo")
	ch2 := hub.Subscribe("owner/repo")

	hub.Notify("owner/repo", "reload")

	select {
	case msg := <-ch1:
		if msg != "reload" {
			t.Errorf("expected 'reload', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1 did not receive message")
	}

	select {
	case msg := <-ch2:
		if msg != "reload" {
			t.Errorf("expected 'reload', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch2 did not receive message")
	}
}

func TestSSEHub_notify_only_sends_to_matching_repo(t *testing.T) {
	hub := NewSSEHub()
	ch1 := hub.Subscribe("owner/repo1")
	ch2 := hub.Subscribe("owner/repo2")

	hub.Notify("owner/repo1", "reload")

	select {
	case <-ch1:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1 should have received message")
	}

	select {
	case <-ch2:
		t.Error("ch2 should not have received message")
	case <-time.After(50 * time.Millisecond):
		// expected - no message
	}
}

func TestSSEHub_notify_skips_slow_clients(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe("owner/repo")

	// Fill the channel buffer (size 10)
	for i := 0; i < 15; i++ {
		hub.Notify("owner/repo", "reload")
	}

	// Should not block or panic
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 10 {
		t.Errorf("expected 10 messages (buffer size), got %d", count)
	}
}

func TestVerifySignature_valid_signature(t *testing.T) {
	payload := []byte(`{"test": "data"}`)
	secret := "mysecret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !verifySignature(payload, signature, secret) {
		t.Error("expected valid signature to pass")
	}
}

func TestVerifySignature_invalid_signature(t *testing.T) {
	payload := []byte(`{"test": "data"}`)
	secret := "mysecret"

	if verifySignature(payload, "sha256=invalid", secret) {
		t.Error("expected invalid signature to fail")
	}
}

func TestVerifySignature_missing_prefix(t *testing.T) {
	payload := []byte(`{"test": "data"}`)
	secret := "mysecret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil)) // missing sha256= prefix

	if verifySignature(payload, signature, secret) {
		t.Error("expected signature without prefix to fail")
	}
}

func TestVerifySignature_wrong_secret(t *testing.T) {
	payload := []byte(`{"test": "data"}`)
	secret := "mysecret"
	wrongSecret := "wrongsecret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if verifySignature(payload, signature, wrongSecret) {
		t.Error("expected wrong secret to fail")
	}
}

func TestHandleWebhook_rejects_non_post(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleWebhook_ignores_non_push_events(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "ping")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != `{"status":"ignored"}` {
		t.Errorf("expected ignored status, got %s", rec.Body.String())
	}
}

func TestHandleWebhook_validates_signature_when_secret_configured(t *testing.T) {
	app := newTestApp()
	app.cfg.WebhookSecret = "mysecret"

	payload := []byte(`{"repository": {"full_name": "owner/repo"}, "commits": []}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestHandleWebhook_accepts_valid_signature(t *testing.T) {
	app := newTestApp()
	app.cfg.WebhookSecret = "mysecret"

	payload := []byte(`{"repository": {"full_name": "owner/repo"}, "commits": []}`)
	mac := hmac.New(sha256.New, []byte("mysecret"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleWebhook_notifies_on_kanban_change(t *testing.T) {
	app := newTestApp()
	app.cfg.KanbanPath = "kanban.md"
	ch := app.sseHub.Subscribe("owner/repo")

	event := PushEvent{
		Repository: struct {
			FullName string `json:"full_name"`
		}{FullName: "owner/repo"},
		Commits: []struct {
			Added    []string `json:"added"`
			Modified []string `json:"modified"`
			Removed  []string `json:"removed"`
		}{
			{Modified: []string{"kanban.md"}},
		},
	}
	payload, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	select {
	case msg := <-ch:
		if msg != "reload" {
			t.Errorf("expected 'reload', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected notification on kanban change")
	}
}

func TestHandleWebhook_notifies_on_archive_change(t *testing.T) {
	app := newTestApp()
	app.cfg.ArchivePath = "archive.md"
	ch := app.sseHub.Subscribe("owner/repo")

	event := PushEvent{
		Repository: struct {
			FullName string `json:"full_name"`
		}{FullName: "owner/repo"},
		Commits: []struct {
			Added    []string `json:"added"`
			Modified []string `json:"modified"`
			Removed  []string `json:"removed"`
		}{
			{Added: []string{"archive.md"}},
		},
	}
	payload, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	select {
	case msg := <-ch:
		if msg != "reload" {
			t.Errorf("expected 'reload', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected notification on archive change")
	}
}

func TestHandleWebhook_no_notification_for_unrelated_files(t *testing.T) {
	app := newTestApp()
	app.cfg.KanbanPath = "kanban.md"
	app.cfg.ArchivePath = "archive.md"
	ch := app.sseHub.Subscribe("owner/repo")

	event := PushEvent{
		Repository: struct {
			FullName string `json:"full_name"`
		}{FullName: "owner/repo"},
		Commits: []struct {
			Added    []string `json:"added"`
			Modified []string `json:"modified"`
			Removed  []string `json:"removed"`
		}{
			{Modified: []string{"README.md", "other.txt"}},
		},
	}
	payload, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	select {
	case <-ch:
		t.Error("should not notify for unrelated file changes")
	case <-time.After(50 * time.Millisecond):
		// expected - no notification
	}
}

func TestHandleWebhook_case_insensitive_repo_matching(t *testing.T) {
	app := newTestApp()
	app.cfg.KanbanPath = "kanban.md"
	ch := app.sseHub.Subscribe("owner/repo") // lowercase

	event := PushEvent{
		Repository: struct {
			FullName string `json:"full_name"`
		}{FullName: "Owner/Repo"}, // mixed case
		Commits: []struct {
			Added    []string `json:"added"`
			Modified []string `json:"modified"`
			Removed  []string `json:"removed"`
		}{
			{Modified: []string{"kanban.md"}},
		},
	}
	payload, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	app.handleWebhook(rec, req)

	select {
	case msg := <-ch:
		if msg != "reload" {
			t.Errorf("expected 'reload', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected notification with case-insensitive repo matching")
	}
}

func newTestApp() *App {
	return &App{
		cfg:    config.Config{},
		sseHub: NewSSEHub(),
	}
}
