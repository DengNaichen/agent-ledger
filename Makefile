.PHONY: build test web

build: web
	go build -o agent-ledger ./cmd/agent-ledger

test: web
	go test ./...

web:
	npm --prefix web run build

