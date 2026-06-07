package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func geminiResponse(text string) []byte {
	b, _ := json.Marshal(map[string]any{
		"candidates": []map[string]any{
			{"content": map[string]any{
				"parts": []map[string]any{
					{"text": text},
				},
			}},
		},
	})
	return b
}

func TestGeminiVision(t *testing.T) {
	const testKey = "test-api-key-vision"
	imgData := []byte("fakeimagedata")
	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != testKey {
			t.Errorf("x-goog-api-key = %q, want %q", got, testKey)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		raw, _ := json.Marshal(body)
		s := string(raw)
		if !strings.Contains(s, "inline_data") {
			t.Errorf("body missing inline_data: %s", s)
		}
		if !strings.Contains(s, imgB64) {
			t.Errorf("body missing image base64: %s", s)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(geminiResponse("motorcycle"))
	}))
	defer srv.Close()

	c := &geminiClient{
		http:  srv.Client(),
		base:  srv.URL,
		key:   testKey,
		model: "gemini-pro-vision",
	}

	got, err := c.Vision(context.Background(), "what is this?", []Image{
		{MediaType: "image/png", Data: imgData},
	}, Options{})
	if err != nil {
		t.Fatalf("Vision error: %v", err)
	}
	if got != "motorcycle" {
		t.Errorf("Vision = %q, want %q", got, "motorcycle")
	}
}

func TestGeminiTranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(geminiResponse("forty two"))
	}))
	defer srv.Close()

	c := &geminiClient{
		http:  srv.Client(),
		base:  srv.URL,
		key:   "test-key",
		model: "gemini-pro",
	}

	got, err := c.Transcribe(context.Background(), []byte("audiodata"), "audio.wav", Options{})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if got != "forty two" {
		t.Errorf("Transcribe = %q, want %q", got, "forty two")
	}
}
