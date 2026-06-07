package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIVision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("auth %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		if !strings.Contains(string(raw), "data:image/png;base64,AQ==") {
			t.Errorf("image not embedded: %s", raw)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ab12"}}]}`)
	}))
	defer srv.Close()

	c := &openAIClient{http: &http.Client{Timeout: 5 * time.Second}, base: srv.URL, key: "key", model: "gpt-4o", maxTokens: 100}
	got, err := c.Vision(context.Background(), "read", []Image{{MediaType: "image/png", Data: []byte{1}}}, Options{})
	if err != nil || got != "ab12" {
		t.Fatalf("got %q err %v", got, err)
	}
}

const maxMultipartMem = 1 << 20 // 1 MiB — bounded, used only in tests

func TestOpenAITranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(maxMultipartMem); err != nil { //nolint:gosec // G120: httptest server, bounded by maxMultipartMem constant, not a real HTTP handler
			t.Errorf("parse multipart: %v", err)
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("model %q", r.FormValue("model"))
		}
		_, _ = io.WriteString(w, `{"text":"the secret is 42"}`)
	}))
	defer srv.Close()

	c := &openAIClient{http: &http.Client{Timeout: 5 * time.Second}, base: srv.URL, key: "key", model: "gpt-4o", audioModel: "whisper-1"}
	got, err := c.Transcribe(context.Background(), []byte("RIFF"), "a.wav", Options{})
	if err != nil || got != "the secret is 42" {
		t.Fatalf("got %q err %v", got, err)
	}
}
