.PHONY: build test web

build: web
	go build -o inftask ./cmd/inftask

test: web
	go test ./...

web:
	npm --prefix web run build

