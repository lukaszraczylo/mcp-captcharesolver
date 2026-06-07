package captcha

import (
	"context"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

func TestSolveRotation(t *testing.T) {
	f := &llm.Fake{VisionResp: `{"choice":3,"rotations":2,"confidence":0.6}`}
	res, err := SolveRotation(context.Background(), f, []byte{1}, "image/png", "rotate the animal upright")
	if err != nil {
		t.Fatal(err)
	}
	if res.Choice != 3 || res.Rotations != 2 {
		t.Fatalf("got %+v", res)
	}
}
