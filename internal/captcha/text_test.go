package captcha

import (
	"context"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

func TestSolveText(t *testing.T) {
	f := &llm.Fake{VisionResp: `{"text":"ab12","confidence":0.9}`}
	res, err := SolveText(context.Background(), f, []byte{1}, "image/png", TextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "ab12" || res.Confidence < 0.89 {
		t.Fatalf("got %+v", res)
	}
	if len(f.VisionCalls) != 1 {
		t.Fatalf("expected 1 vision call")
	}
}

func TestSolveTextRetriesOnGarbage(t *testing.T) {
	f := &llm.Fake{VisionResp: "I cannot read this"}
	_, err := SolveText(context.Background(), f, []byte{1}, "image/png", TextOptions{})
	if err == nil {
		t.Fatal("want error on unparseable output")
	}
	if len(f.VisionCalls) != 2 {
		t.Fatalf("expected 2 attempts (one retry), got %d", len(f.VisionCalls))
	}
}

func TestSolveTextVoting(t *testing.T) {
	f := &llm.Fake{VisionResponses: []string{
		`{"text":"ab12","confidence":0.7}`,
		`{"text":"ab12","confidence":0.7}`,
		`{"text":"ab13","confidence":0.9}`,
	}}
	res, err := SolveText(context.Background(), f, []byte{1}, "image/png", TextOptions{Samples: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "ab12" {
		t.Fatalf("voted text %q, want ab12", res.Text)
	}
	if len(f.VisionCalls) != 3 {
		t.Fatalf("expected 3 vision calls, got %d", len(f.VisionCalls))
	}
}
