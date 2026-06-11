//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startServer starts the captcha-solver-mcp binary in HTTP transport mode,
// returning the base URL (e.g. "http://127.0.0.1:34567") and a stop func.
func startServer(t *testing.T, llmURL string) (string, func()) {
	t.Helper()

	// Pick a free port by asking the kernel.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "../../bin/captcha-solver-mcp")
	cmd.Env = append(cmd.Environ(),
		"CAPTCHA_LLM_PROVIDER=openai-compatible",
		"CAPTCHA_LLM_BASE_URL="+llmURL,
		"CAPTCHA_LLM_API_KEY=test",
		"CAPTCHA_LLM_MODEL=fake",
		"CAPTCHA_TRANSPORT=http",
		"CAPTCHA_HTTP_ADDR="+addr,
		"CAPTCHA_HTTP_PATH=/mcp",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start: %v", err)
	}

	// Wait for /healthz to respond.
	base := "http://" + addr
	if !waitHealthy(base+"/healthz", 5*time.Second) {
		_ = cmd.Process.Kill()
		cancel()
		t.Fatalf("server failed to become healthy at %s", base)
	}

	return base, func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
		cancel()
	}
}

func waitHealthy(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestHealthz(t *testing.T) {
	llmSrv := startFakeLLM()
	defer llmSrv.Close()

	base, stop := startServer(t, llmSrv.URL)
	defer stop()

	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "ok" {
		t.Fatalf("healthz body = %q, want %q", body, "ok")
	}
}

func TestSolveGridOverHTTP(t *testing.T) {
	llmSrv := startFakeLLM()
	defer llmSrv.Close()

	base, stop := startServer(t, llmSrv.URL)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpoint, _ := url.Parse(base + "/mcp")
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-http", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint.String()}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "solve_grid_captcha",
		Arguments: map[string]any{
			"screenshot":   "iVBORw0KGgo=",
			"instruction":  "select all buses",
			"rows":         3,
			"cols":         3,
			"image_width":  300,
			"image_height": 300,
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got struct {
		Tiles []int `json:"tiles"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw=%s", err, raw)
	}
	want := []int{0, 1, 2}
	if len(got.Tiles) != len(want) {
		t.Fatalf("tiles = %v, want %v", got.Tiles, want)
	}
	for i := range want {
		if got.Tiles[i] != want[i] {
			t.Fatalf("tiles = %v, want %v", got.Tiles, want)
		}
	}
}
