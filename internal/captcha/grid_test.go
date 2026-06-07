package captcha

import (
	"context"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

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
