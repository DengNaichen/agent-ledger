package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"github.com/DengNaichen/agent-ledger/internal/httpapi"
	"github.com/DengNaichen/agent-ledger/internal/mcpserver"
	webui "github.com/DengNaichen/agent-ledger/internal/web"
)

func main() {
	data := flag.String("data", "agent-ledger.db", "SQLite database path")
	addr := flag.String("addr", "127.0.0.1:8787", "HTTP listen address")
	prefix := flag.String("prefix", "AL", "Issue identifier prefix")
	var repos []string
	flag.Func("github-repo", "Sync pull requests from this GitHub repository, as owner/name (repeatable; needs GITHUB_TOKEN)", func(repo string) error {
		repos = append(repos, repo)
		return nil
	})
	linkback := flag.Bool("github-linkback", false, "Comment the issue title and description on open pull requests when they are first linked")
	flag.Parse()
	token := os.Getenv("GITHUB_TOKEN")
	if len(repos) > 0 && token == "" {
		log.Fatal("GITHUB_TOKEN is required with -github-repo")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	service, err := app.Open(*data, *prefix)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()

	go backupDaily(ctx, service)
	if len(repos) > 0 {
		go app.NewGitHubSync(service, repos, token, *linkback).Run(ctx)
	}
	serve(ctx, *addr, service)
}

func backupDaily(ctx context.Context, service *app.Service) {
	for {
		path, err := service.Backup(ctx)
		if err != nil {
			log.Printf("backup failed: %v", err)
		} else if path != "" {
			log.Printf("backed up to %s", path)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
	}
}

func serve(ctx context.Context, addr string, service *app.Service) {
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(service, webui.Handler(), mcpserver.Handler(service)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("Agent Ledger listening on http://%s (MCP at /mcp)", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
