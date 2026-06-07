//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// startFakeLLM returns an OpenAI-compatible endpoint that always selects tiles 0,1,2.
func startFakeLLM() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"tiles\":[0,1,2],\"confidence\":0.95,\"recheck_recommended\":false}"}}]}`)
		case "/v1/audio/transcriptions":
			_, _ = io.WriteString(w, `{"text":"fortytwo"}`)
		default:
			http.NotFound(w, r)
		}
	}))
}
