//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/octoberswimmer/masc"
)

func main() {
	masc.SetTitle("AER Sales Kanban")
	js.Global().Set("startWithDiv", jsStartFunc())
	select {}
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
