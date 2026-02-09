package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]map[chan SSEEvent]struct{} // repo -> set of channels
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[chan SSEEvent]struct{}),
	}
}

func (h *SSEHub) Subscribe(repo string) chan SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan SSEEvent, 10)
	if h.clients[repo] == nil {
		h.clients[repo] = make(map[chan SSEEvent]struct{})
	}
	h.clients[repo][ch] = struct{}{}
	return ch
}

func (h *SSEHub) Unsubscribe(repo string, ch chan SSEEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[repo] != nil {
		delete(h.clients[repo], ch)
		if len(h.clients[repo]) == 0 {
			delete(h.clients, repo)
		}
	}
	close(ch)
}

type SSEEvent struct {
	Type    string
	HeadOID string
}

func (h *SSEHub) Notify(repo string, headOID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[repo]; ok {
		event := SSEEvent{Type: "reload", HeadOID: headOID}
		for ch := range clients {
			select {
			case ch <- event:
			default:
				// Client too slow, skip
			}
		}
	}
}

type PushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"` // New HEAD SHA after the push
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

func (a *App) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if a.cfg.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, signature, a.cfg.WebhookSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "push" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	repo := strings.ToLower(event.Repository.FullName)
	if repo == "" {
		http.Error(w, "missing repository", http.StatusBadRequest)
		return
	}

	kanbanPath := a.cfg.KanbanPath
	archivePath := a.cfg.ArchivePath
	changed := false

	for _, commit := range event.Commits {
		allFiles := append(append(commit.Added, commit.Modified...), commit.Removed...)
		for _, file := range allFiles {
			if file == kanbanPath || file == archivePath {
				changed = true
				break
			}
		}
		if changed {
			break
		}
	}

	if changed {
		log.Printf("Webhook: %s changed (commit %s), notifying clients", repo, event.After)
		a.sseHub.Notify(repo, event.After)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (a *App) handleSSE(w http.ResponseWriter, r *http.Request) {
	session, err := a.readSession(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	repo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("repo")))
	if repo == "" {
		http.Error(w, "missing repo parameter", http.StatusBadRequest)
		return
	}

	if !a.verifyRepoAccess(r.Context(), session.AccessToken, repo) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := a.sseHub.Subscribe(repo)
	defer a.sseHub.Unsubscribe(repo, ch)

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"repo\":%q}\n\n", repo)
	flusher.Flush()

	// Heartbeat ticker to keep connection alive
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: {\"repo\":%q,\"head_oid\":%q}\n\n", event.Type, repo, event.HeadOID)
			flusher.Flush()
		case <-heartbeat.C:
			// SSE comment format keeps connection alive without triggering client events
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (a *App) verifyRepoAccess(ctx context.Context, token, repo string) bool {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return false
	}
	owner, name := parts[0], parts[1]

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func verifySignature(payload []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}
