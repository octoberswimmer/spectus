spectus: build
	go build -o bin/spectus .

run:
	go run .

dev:
	gin --all --pre-build make build --make run

build: static/main.wasm static/wasm_exec.js

static/main.wasm: $(shell find masc -type f) go.mod go.sum
	env GOOS=js GOARCH=wasm go build -o static/main.wasm ./masc

static/wasm_exec.js:
	mkdir -p static
	@if [ -f "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" static/wasm_exec.js; \
	else \
		cp "$(shell go env GOROOT)/misc/wasm/wasm_exec.js" static/wasm_exec.js; \
	fi

deps:
	go get ./...

clean:
	-rm -f static/main.wasm static/wasm_exec.js

.PHONY: build deps clean dev run spectus
