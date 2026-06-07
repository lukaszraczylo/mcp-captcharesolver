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

func TestFakeVisionResponsesCycle(t *testing.T) {
	f := &Fake{VisionResponses: []string{"a", "b", "c"}}
	want := []string{"a", "b", "c", "c", "c"} // clamps to last beyond slice length
	for i, w := range want {
		got, err := f.Vision(context.Background(), "p", nil, Options{})
		if err != nil {
			t.Fatalf("call %d err %v", i, err)
		}
		if got != w {
			t.Fatalf("call %d got %q want %q", i, got, w)
		}
	}
	if len(f.VisionCalls) != len(want) {
		t.Fatalf("recorded %d calls, want %d", len(f.VisionCalls), len(want))
	}
}

func TestFakeVisionFallsBackToVisionResp(t *testing.T) {
	f := &Fake{VisionResp: "single"}
	got, err := f.Vision(context.Background(), "p", nil, Options{})
	if err != nil || got != "single" {
		t.Fatalf("got %q err %v, want single", got, err)
	}
}
