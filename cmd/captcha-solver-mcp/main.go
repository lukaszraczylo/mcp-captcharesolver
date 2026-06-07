// Command captcha-solver-mcp runs the captcha solver MCP server over stdio.
package main

import (
	"context"
	"log"

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
	server := mcp.NewServer(&mcp.Implementation{Name: "captcha-solver-mcp", Version: "v0.1.0"}, nil)
	mcpserver.New(client, cfg.Provider, cfg.Model).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
