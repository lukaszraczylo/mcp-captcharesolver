package mcpserver

import (
	"context"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

func TestGridHandler(t *testing.T) {
	h := New(&llm.Fake{VisionResp: `{"tiles":[1,2],"confidence":0.7,"recheck_recommended":false}`}, "anthropic", "claude-opus-4-8")
	_, out, err := h.solveGrid(context.Background(), nil, SolveGridIn{
		Screenshot: pngB64(), Instruction: "buses", Rows: 3, Cols: 3, ImageWidth: 300, ImageHeight: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Tiles) != 2 || len(out.Centroids) != 2 {
		t.Fatalf("out=%+v", out)
	}
}

func TestDetectHandler(t *testing.T) {
	h := New(&llm.Fake{}, "anthropic", "claude-opus-4-8")
	_, out, err := h.detect(context.Background(), nil, DetectIn{HTML: `<div class="g-recaptcha" data-sitekey="k"></div>`})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Detected) != 1 || out.Detected[0].Type != "recaptcha_v2" {
		t.Fatalf("out=%+v", out)
	}
}

// pngB64 returns a 1px PNG as base64 (valid magic bytes are enough for decode).
func pngB64() string { return "iVBORw0KGgo=" } // "iVBORw..." decodes to PNG magic prefix
