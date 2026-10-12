.PHONY: build test web

build: web
	go build -o inftask ./cmd/inftask

test: web
	npm --prefix web run lint
	npm --prefix web run test
	go test ./...

web:
	npm --prefix web run build

