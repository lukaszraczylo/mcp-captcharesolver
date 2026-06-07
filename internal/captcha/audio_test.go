package captcha

import (
	"context"
	"errors"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

func TestSolveAudio(t *testing.T) {
	f := &llm.Fake{TranscribeResp: "  forty two  "}
	res, err := SolveAudio(context.Background(), f, []byte("RIFF"), "a.wav", "en")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "forty two" {
		t.Fatalf("got %q", res.Text)
	}
}

func TestSolveAudioUnsupported(t *testing.T) {
	f := &llm.Fake{TranscribeErr: llm.ErrUnsupported{Provider: "anthropic"}}
	_, err := SolveAudio(context.Background(), f, []byte{1}, "a.wav", "")
	var u llm.ErrUnsupported
	if !errors.As(err, &u) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
