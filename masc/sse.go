//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/octoberswimmer/masc"
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

		url := "/events?repo=" + js.Global().Call("encodeURIComponent", repo).String()
		eventSource := js.Global().Get("EventSource").New(url)

		msgChan := make(chan masc.Msg, 1)

		onReload := js.FuncOf(func(this js.Value, args []js.Value) any {
			select {
			case msgChan <- SSEReload{Repo: repo}:
			default:
			}
			return nil
		})

		onError := js.FuncOf(func(this js.Value, args []js.Value) any {
			// EventSource auto-reconnects, but if it fails completely we should restart
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

		eventSource.Call("addEventListener", "reload", onReload)
		eventSource.Call("addEventListener", "error", onError)

		msg := <-msgChan

		eventSource.Call("close")
		onReload.Release()
		onError.Release()

		return msg
	}
}
