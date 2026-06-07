package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnthropicVision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "key" || r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("headers %v", r.Header)
		}
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "temperature") {
			t.Errorf("temperature must NOT be sent to anthropic: %s", raw)
		}
		if !strings.Contains(string(raw), `"type":"base64"`) {
			t.Errorf("image block missing: %s", raw)
		}
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"crosswalk"}]}`)
	}))
	defer srv.Close()

	c := &anthropicClient{http: &http.Client{Timeout: 5 * time.Second}, base: srv.URL, key: "key", model: "claude-opus-4-8", maxTokens: 100}
	got, err := c.Vision(context.Background(), "which tiles", []Image{{MediaType: "image/png", Data: []byte{2}}}, Options{Temperature: 0.7})
	if err != nil || got != "crosswalk" {
		t.Fatalf("got %q err %v", got, err)
	}
	_ = json.Marshal // keep import
}

func TestAnthropicTranscribeUnsupported(t *testing.T) {
	c := &anthropicClient{model: "claude-opus-4-8"}
	_, err := c.Transcribe(context.Background(), []byte{1}, "a.mp3", Options{})
	var u ErrUnsupported
	if !errors.As(err, &u) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
