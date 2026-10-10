package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"github.com/DengNaichen/agent-ledger/internal/httpapi"
	"github.com/DengNaichen/agent-ledger/internal/mcpserver"
	webui "github.com/DengNaichen/agent-ledger/internal/web"
)

func main() {
	mode := flag.String("mode", "serve", "serve or mcp")
	data := flag.String("data", "agent-ledger.db", "SQLite database path")
	addr := flag.String("addr", "127.0.0.1:8787", "HTTP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	service, err := app.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()

	switch *mode {
	case "mcp":
		if err := mcpserver.Run(ctx, service); err != nil {
			log.Fatal(err)
		}
	case "serve":
		serve(ctx, *addr, service)
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

func serve(ctx context.Context, addr string, service *app.Service) {
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(service, webui.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("Agent Ledger listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
