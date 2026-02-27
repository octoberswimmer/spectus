//go:build js && wasm

package main

import (
	"encoding/json"
	"strings"
	"syscall/js"
	"time"

	"github.com/octoberswimmer/masc"
)

type sseReloadData struct {
	Repo    string `json:"repo"`
	HeadOID string `json:"head_oid"`
}

var (
	activeSSE     js.Value
	activeSSERepo string
	sseCallbacks  []js.Func
)

func (p *Program) sseListenCmd() masc.Cmd {
	return p.sseListenWithBackoff(0)
}

func (p *Program) sseListenWithBackoff(retryDelay int) masc.Cmd {
	repo := strings.ToLower(strings.TrimSpace(p.selectedRepo))
	if repo == "" {
		return nil
	}

	return func() masc.Msg {
		// Wait for retry delay if specified
		if retryDelay > 0 {
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}

		// Reuse existing connection if it's for the same repo and still open
		if activeSSERepo == repo && !activeSSE.IsUndefined() && !activeSSE.IsNull() {
			state := activeSSE.Get("readyState").Int()
			if state != 2 { // Not CLOSED
				// Connection still active, just wait for next event
				msgChan := make(chan masc.Msg, 1)
				onReload := js.FuncOf(func(this js.Value, args []js.Value) any {
					headOID := ""
					if len(args) > 0 {
						dataStr := args[0].Get("data").String()
						var data sseReloadData
						if json.Unmarshal([]byte(dataStr), &data) == nil {
							headOID = data.HeadOID
						}
					}
					select {
					case msgChan <- SSEReload{Repo: repo, HeadOID: headOID}:
					default:
					}
					return nil
				})
				activeSSE.Call("addEventListener", "reload", onReload, map[string]interface{}{"once": true})
				msg := <-msgChan
				onReload.Release()
				return msg
			}
		}

		// Close any existing connection for different repo
		closeActiveSSE()

		url := "/events?repo=" + js.Global().Call("encodeURIComponent", repo).String()
		eventSource := js.Global().Get("EventSource").New(url)
		activeSSE = eventSource
		activeSSERepo = repo

		msgChan := make(chan masc.Msg, 1)

		onReload := js.FuncOf(func(this js.Value, args []js.Value) any {
			headOID := ""
			if len(args) > 0 {
				dataStr := args[0].Get("data").String()
				var data sseReloadData
				if json.Unmarshal([]byte(dataStr), &data) == nil {
					headOID = data.HeadOID
				}
			}
			select {
			case msgChan <- SSEReload{Repo: repo, HeadOID: headOID}:
			default:
			}
			return nil
		})

		onError := js.FuncOf(func(this js.Value, args []js.Value) any {
			state := eventSource.Get("readyState").Int()
			if state == 2 { // CLOSED
				// Calculate next retry delay with exponential backoff (max 30 seconds)
				nextDelay := retryDelay * 2
				if nextDelay < 1000 {
					nextDelay = 1000
				}
				if nextDelay > 30000 {
					nextDelay = 30000
				}
				select {
				case msgChan <- SSEError{Repo: repo, RetryDelay: nextDelay}:
				default:
				}
			}
			return nil
		})

		sseCallbacks = []js.Func{onReload, onError}
		eventSource.Call("addEventListener", "reload", onReload)
		eventSource.Call("addEventListener", "error", onError)

		return <-msgChan
	}
}

func closeActiveSSE() {
	if !activeSSE.IsUndefined() && !activeSSE.IsNull() {
		activeSSE.Call("close")
	}
	for _, cb := range sseCallbacks {
		cb.Release()
	}
	sseCallbacks = nil
	activeSSE = js.Undefined()
	activeSSERepo = ""
}

const sessionCheckInterval = 5 * time.Minute

// sessionCheckTickCmd schedules the next session check after a delay
func (p *Program) sessionCheckTickCmd() masc.Cmd {
	return masc.Tick(sessionCheckInterval, func(t time.Time) masc.Msg {
		return SessionCheckTick{}
	})
}

// sessionCheckCmd performs a lightweight API call to validate the token
func (p *Program) sessionCheckCmd() masc.Cmd {
	token := p.token
	return func() masc.Msg {
		client := &GraphQLClient{Token: token}
		_, err := fetchViewer(client)
		if err != nil {
			return SessionCheckResult{Unauthorized: isUnauthorized(err)}
		}
		return SessionCheckResult{Unauthorized: false}
	}
}
