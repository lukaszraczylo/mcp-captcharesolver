package llm

import (
	"context"
	"testing"
)

func TestFakeClient(t *testing.T) {
	f := &Fake{VisionResp: "hello", TranscribeResp: "spoken"}
	got, err := f.Vision(context.Background(), "p", []Image{{MediaType: "image/png", Data: []byte{1}}}, Options{})
	if err != nil || got != "hello" {
		t.Fatalf("vision got %q err %v", got, err)
	}
	if len(f.VisionCalls) != 1 || f.VisionCalls[0].Prompt != "p" {
		t.Fatalf("vision call not recorded: %+v", f.VisionCalls)
	}
	tr, err := f.Transcribe(context.Background(), []byte{1}, "audio.mp3", Options{Language: "en"})
	if err != nil || tr != "spoken" {
		t.Fatalf("transcribe got %q err %v", tr, err)
	}
}
