//go:build e2e && live

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestLiveSolveGrid drives the whole stack against a real vision model: it
// launches the built MCP server, speaks MCP over stdio, and asserts the model
// selects the solid-red tile row of a generated grid. Opt-in only: requires
// build tags e2e,live AND CAPTCHA_E2E_LIVE=1, plus ambient LLM provider/model
// config which is inherited by the child via cmd.Environ().
func TestLiveSolveGrid(t *testing.T) {
	if os.Getenv("CAPTCHA_E2E_LIVE") != "1" {
		t.Skip("CAPTCHA_E2E_LIVE != 1; skipping live LLM e2e")
	}
	if os.Getenv("CAPTCHA_LLM_PROVIDER") == "" || os.Getenv("CAPTCHA_LLM_MODEL") == "" {
		t.Skip("CAPTCHA_LLM_PROVIDER / CAPTCHA_LLM_MODEL not set; skipping live LLM e2e")
	}

	const (
		size = 300
		rows = 3
		cols = 3
	)
	b64 := genRedTopRowPNG(t, size)

	cmd := exec.Command("../../bin/captcha-solver-mcp")
	cmd.Env = cmd.Environ()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-live", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "solve_grid_captcha",
		Arguments: map[string]any{
			"screenshot":   b64,
			"instruction":  "Select every tile that is solid red.",
			"rows":         rows,
			"cols":         cols,
			"image_width":  size,
			"image_height": size,
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
		t.Fatalf("marshal structured: %v", err)
	}
	var out struct {
		Tiles      []int   `json:"tiles"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}

	t.Logf("model returned tiles=%v confidence=%.3f", out.Tiles, out.Confidence)

	if !tilesContain(out.Tiles, 0, 1, 2) {
		t.Fatalf("tiles %v missing red row {0,1,2}", out.Tiles)
	}
	for _, got := range out.Tiles {
		if got != 0 && got != 1 && got != 2 {
			t.Logf("model selected extra (non-red) tile %d", got)
		}
	}
}

// genRedTopRowPNG builds a sizeXsize PNG whose top third (tiles 0,1,2) is solid
// bright red and the rest light gray, then returns it base64-encoded.
func genRedTopRowPNG(t *testing.T, size int) string {
	t.Helper()
	red := color.RGBA{R: 220, G: 30, B: 30, A: 255}
	gray := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	topRow := size / 3
	for y := 0; y < size; y++ {
		c := gray
		if y < topRow {
			c = red
		}
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// tilesContain reports whether got contains every value in want.
func tilesContain(got []int, want ...int) bool {
	set := make(map[int]struct{}, len(got))
	for _, g := range got {
		set[g] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}
