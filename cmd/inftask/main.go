package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/DengNaichen/inftask/internal/mcpserver"
	"github.com/DengNaichen/inftask/internal/ucauth"
	webui "github.com/DengNaichen/inftask/internal/web"
)

func main() {
	data := flag.String("data", "inftask.db", "SQLite database path")
	addr := flag.String("addr", "127.0.0.1:8787", "HTTP listen address")
	var repos []app.GitHubRepo
	flag.Func("github-repo", "Sync pull requests from a GitHub repository into a workspace, as <slug>=<owner>/<name> (repeatable; needs GITHUB_TOKEN)", gitHubRepoFlag(&repos))
	linkback := flag.Bool("github-linkback", false, "Comment the issue title and description on open pull requests when they are first linked")
	flag.Parse()
	token := os.Getenv("GITHUB_TOKEN")
	if len(repos) > 0 && token == "" {
		log.Fatal("GITHUB_TOKEN is required with -github-repo")
	}

	auth, err := cloudAuth(*addr)
	if err != nil {
		log.Fatal(err)
	}
	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if webhookSecret != "" && len(repos) == 0 {
		log.Fatal("GITHUB_WEBHOOK_SECRET requires at least one -github-repo")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	service, err := app.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()

	go backupDaily(ctx, service)
	handler := httpapi.New(service, webui.Handler(), mcpserver.Handler(service))
	if auth != nil {
		handler = auth.Handler(handler)
	}
	if len(repos) > 0 {
		sync := app.NewGitHubSync(service, repos, token, *linkback)
		go sync.Run(ctx)
		if webhookSecret != "" {
			mux := http.NewServeMux()
			mux.Handle("POST /webhooks/github", httpapi.GitHubWebhook(sync, webhookSecret))
			mux.Handle("/", handler)
			handler = mux
		}
	}
	serve(ctx, *addr, handler)
}

// gitHubRepoFlag parses a -github-repo value, which binds a repository to the
// workspace whose issues its pull requests link to.
func gitHubRepoFlag(repos *[]app.GitHubRepo) func(string) error {
	return func(value string) error {
		workspace, repo, found := strings.Cut(value, "=")
		if !found || workspace == "" {
			return errors.New("use <slug>=<owner>/<name>, such as default=DengNaichen/inftask")
		}
		*repos = append(*repos, app.GitHubRepo{Workspace: workspace, Repo: repo})
		return nil
	}
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

func serve(ctx context.Context, addr string, handler http.Handler) {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("InfTask listening on http://%s (MCP at /w/<slug>/mcp)", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func cloudAuth(addr string) (*ucauth.Auth, error) {
	cfg := ucauth.Config{
		BaseURL: os.Getenv("INFONE_UC_BASE_URL"), ClientID: os.Getenv("INFONE_UC_CLIENT_ID"), ClientSecret: os.Getenv("INFONE_UC_CLIENT_SECRET"),
		PublicURL: os.Getenv("INFTASK_PUBLIC_URL"), Issuer: os.Getenv("INFONE_UC_JWT_ISSUER"),
	}
	share := os.Getenv("INFONE_SHARE_DIR")
	if cfg.BaseURL == "" && cfg.ClientID == "" && cfg.ClientSecret == "" && cfg.PublicURL == "" && cfg.Issuer == "" && share == "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil || !(strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()) {
			return nil, errors.New("UC authentication is required for a non-loopback listener")
		}
		return nil, nil
	}
	if share == "" {
		return nil, errors.New("INFONE_SHARE_DIR is required for UC authentication")
	}
	cfg.PublicKeyPath = filepath.Join(share, "infone.pub")
	return ucauth.New(cfg)
}
