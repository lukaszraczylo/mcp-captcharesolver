//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSolveGridOverStdio(t *testing.T) {
	llmSrv := startFakeLLM()
	defer llmSrv.Close()

	cmd := exec.Command("../../bin/captcha-solver-mcp")
	cmd.Env = append(cmd.Environ(),
		"CAPTCHA_LLM_PROVIDER=openai-compatible",
		"CAPTCHA_LLM_BASE_URL="+llmSrv.URL,
		"CAPTCHA_LLM_API_KEY=test",
		"CAPTCHA_LLM_MODEL=fake",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
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
	assertTiles(t, res, []int{0, 1, 2})
}

// assertTiles unmarshals the structured tool result and compares tiles.
func assertTiles(t *testing.T, res *mcp.CallToolResult, want []int) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured: %v", err)
	}
	var out struct {
		Tiles []int `json:"tiles"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	if len(out.Tiles) != len(want) {
		t.Fatalf("tiles=%v want %v", out.Tiles, want)
	}
	for i := range want {
		if out.Tiles[i] != want[i] {
			t.Fatalf("tiles=%v want %v", out.Tiles, want)
		}
	}
}
