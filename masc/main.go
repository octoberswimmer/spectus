//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/octoberswimmer/masc"
)

func main() {
	masc.SetTitle("Spectus")
	setupBeforeUnload()
	js.Global().Set("startWithDiv", jsStartFunc())
	select {}
}

func setupBeforeUnload() {
	js.Global().Set("hasDirtyChanges", false)
	js.Global().Get("window").Call("addEventListener", "beforeunload", js.FuncOf(func(this js.Value, args []js.Value) any {
		if js.Global().Get("hasDirtyChanges").Bool() {
			event := args[0]
			event.Call("preventDefault")
			return "You have uncommitted changes. Are you sure you want to leave?"
		}
		return nil
	}))
}

func jsStartFunc() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		node := args[0]
		sessionJSON := args[1].String()
		configJSON := args[2].String()

		cfg := ClientConfig{}
		if configJSON != "" {
			_ = json.Unmarshal([]byte(configJSON), &cfg)
		}

		program := NewProgram(cfg)
		pgm := masc.NewProgram(program, masc.RenderTo(node))

		go func() {
			pgm.Send(SetSession{Session: sessionJSON})
		}()

		go func() {
			_, err := pgm.Run()
			if err != nil {
				panic(err)
			}
		}()

		return nil
	})
}
