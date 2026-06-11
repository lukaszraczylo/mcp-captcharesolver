// Command captcha-solver-mcp runs the captcha solver MCP server.
//
// Transport is selected at startup by CAPTCHA_TRANSPORT:
//
//	stdio  — default. Speaks MCP over stdin/stdout for desktop clients.
//	http   — serves MCP StreamableHTTP at CAPTCHA_HTTP_PATH (default /mcp)
//	         and /healthz on CAPTCHA_HTTP_ADDR (default :8080).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/config"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
	mcpserver "github.com/lukaszraczylo/captcha-solver-mcp/internal/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	client, err := llm.New(cfg)
	if err != nil {
		log.Fatalf("llm: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "captcha-solver-mcp", Version: version()}, nil)
	handlers := mcpserver.New(client, cfg.Provider, cfg.Model)
	handlers.Register(server)

	switch cfg.Transport {
	case "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("server: %v", err)
		}
	case "http":
		runHTTP(server, cfg)
	default:
		// Validated by config.Load; unreachable.
		log.Fatalf("transport: unsupported %q", cfg.Transport)
	}
}

func runHTTP(server *mcp.Server, cfg config.Config) {
	mux := http.NewServeMux()

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless: true,
		},
	)
	mux.Handle(cfg.HTTPPath, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0, // 0 = no limit on the body; LLM-bound tool calls can be long.
		WriteTimeout:      0, // long-lived SSE/streamable responses.
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("captcha-solver-mcp http listening on %s (path=%s)", cfg.HTTPAddr, cfg.HTTPPath)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("captcha-solver-mcp shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Fatalf("shutdown: %v", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}
}

func version() string {
	if v := os.Getenv("CAPTCHA_VERSION"); v != "" {
		return v
	}
	return "v0.1.0"
}
