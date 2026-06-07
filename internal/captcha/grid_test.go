package captcha

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// smallPNG returns a tiny valid PNG for exercising preprocessing paths.
func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 6, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestSolveGrid(t *testing.T) {
	f := &llm.Fake{VisionResp: `{"tiles":[0,4,8],"confidence":0.8,"recheck_recommended":true}`}
	res, err := SolveGrid(context.Background(), f, []byte{1}, "image/png", GridOptions{
		Instruction: "select all buses", Rows: 3, Cols: 3, ImageWidth: 300, ImageHeight: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tiles) != 3 || res.Tiles[0] != 0 {
		t.Fatalf("tiles %+v", res.Tiles)
	}
	if len(res.Centroids) != 3 || res.Centroids[1].X != 150 { // tile 4 centre
		t.Fatalf("centroids %+v", res.Centroids)
	}
	if !res.RecheckRecommended {
		t.Fatal("recheck flag lost")
	}
}

func TestSolveGridVoting(t *testing.T) {
	f := &llm.Fake{VisionResponses: []string{
		`{"tiles":[0,1],"confidence":0.9}`,
		`{"tiles":[0,2],"confidence":0.9}`,
		`{"tiles":[0,1],"confidence":0.9}`,
	}}
	res, err := SolveGrid(context.Background(), f, []byte{1}, "image/png", GridOptions{
		Instruction: "select all buses", Rows: 3, Cols: 3, Samples: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 0 in 3/3, 1 in 2/3 (>half), 2 in 1/3 (dropped).
	if len(res.Tiles) != 2 || res.Tiles[0] != 0 || res.Tiles[1] != 1 {
		t.Fatalf("voted tiles %+v, want [0 1]", res.Tiles)
	}
	if len(f.VisionCalls) != 3 {
		t.Fatalf("expected 3 vision calls, got %d", len(f.VisionCalls))
	}
}

func TestSolveGridNoTargets(t *testing.T) {
	f := &llm.Fake{VisionResp: `{"tiles":[],"confidence":0.9,"no_targets":true}`}
	res, err := SolveGrid(context.Background(), f, []byte{1}, "image/png", GridOptions{
		Instruction: "select all buses", Rows: 3, Cols: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.NoTargets {
		t.Fatal("no_targets flag lost")
	}
	if len(res.Tiles) != 0 {
		t.Fatalf("expected empty tiles, got %+v", res.Tiles)
	}
}

func TestSolveGridPreprocessing(t *testing.T) {
	f := &llm.Fake{VisionResp: `{"tiles":[0,4],"confidence":0.8}`}
	res, err := SolveGrid(context.Background(), f, smallPNG(t), "image/png", GridOptions{
		Instruction: "select all buses", Rows: 3, Cols: 3, Annotate: true, Upscale: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tiles) != 2 {
		t.Fatalf("expected 2 tiles, got %+v", res.Tiles)
	}
}
