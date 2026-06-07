# captcha-solver-mcp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go MCP server that acts as a browser-agnostic vision/LLM "brain" for captcha solving and a stealth-artifact generator, fed by the caller's Playwright/Lightpanda.

**Architecture:** Solver-only/brain-only. Tools take images/audio/challenge-screenshots/page-context and return answers, tile-click decisions, and stealth artifacts (JS init scripts, fingerprints, humanized interaction plans). The LLM sits behind a `llm.Client` interface so every handler is unit-testable with a fake client. MCP over stdio via the official Go SDK.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk` v1.2.0 (MCP), stdlib `net/http` for LLM providers (raw REST, no provider SDKs — uniform multi-provider layer), stdlib `image` for grid geometry. E2E: Go test harness driving the real server over stdio + Playwright (Chromium) and Lightpanda against a local fake-captcha page.

**Grounded references (verified this session):**
- Go MCP SDK v1.2.0: `mcp.NewServer(&mcp.Implementation{Name,Version}, nil)`; `mcp.AddTool(server, &mcp.Tool{Name,Description}, handler)` where `handler` is `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`; `server.Run(ctx, &mcp.StdioTransport{})`. `In`/`Out` use `json:"…"` + `jsonschema:"…"` tags; return `(nil, out, nil)` and the SDK fills structured content.
- OpenAI: `POST {base}/v1/chat/completions` with message content parts `{"type":"text","text":…}` and `{"type":"image_url","image_url":{"url":"data:image/png;base64,…"}}`; response `.choices[0].message.content`. Transcription: `POST {base}/v1/audio/transcriptions` multipart fields `model` + `file`; response `.text`.
- Anthropic: `POST {base}/v1/messages`, headers `x-api-key`, `anthropic-version: 2023-06-01`; body `{model,max_tokens,messages:[{role:"user",content:[{type:"text",text},{type:"image",source:{type:"base64",media_type,data}}]}]}`; response `.content[]` first `{type:"text"}`. **Do NOT send `temperature`/`top_p`** (Opus 4.x → 400).

**Quality gates (per project standards):** `golangci-lint run` clean incl. `fieldalignment`; `gosec` applies to `_test.go` too; table-driven tests; build/test locally (no GitHub Actions).

**Conventions used throughout this plan:**
- Module path: `github.com/lukaszraczylo/captcha-solver-mcp`.
- All structs are written with fields ordered largest-alignment-first to satisfy `fieldalignment` (strings = 16 bytes, then `float64`/`int64`/`int` = 8, then `bool` = 1).
- Commit after each task. Conventional-commit prefixes; keep bodies free of the words "breaking"/"feat" unless intended (semver-generator fuzzy-matches commit bodies).

---

## Phase 0 — Scaffold

### Task 0: Module, directories, tooling

**Files:**
- Create: `go.mod`, `.golangci.yml`, `Makefile`, `.env.example`, `README.md` (skeleton)
- Create dir markers via first real files in later tasks.

- [ ] **Step 1: Init module**

Run:
```bash
go mod init github.com/lukaszraczylo/captcha-solver-mcp
go get github.com/modelcontextprotocol/go-sdk/mcp@v1.2.0
```
Expected: `go.mod` + `go.sum` created with the SDK dependency.

- [ ] **Step 2: Write `.golangci.yml`**

```yaml
version: "2"
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - unused
    - gosec
linters-settings:
  govet:
    enable:
      - fieldalignment
issues:
  exclude-rules: []
```

- [ ] **Step 3: Write `Makefile`**

```makefile
.PHONY: build test e2e e2e-live lint tidy
build:
	go build -o bin/captcha-solver-mcp ./cmd/captcha-solver-mcp
test:
	go test ./...
e2e:
	go test -tags e2e ./test/e2e/...
e2e-live:
	CAPTCHA_E2E_LIVE=1 go test -tags e2e,live ./test/e2e/...
lint:
	golangci-lint run
tidy:
	go mod tidy
```

- [ ] **Step 4: Write `.env.example`**

```bash
# Provider: anthropic | openai | openai-compatible | gemini
CAPTCHA_LLM_PROVIDER=anthropic
# Optional endpoint override (local/Ollama/OpenRouter). Empty = provider default.
CAPTCHA_LLM_BASE_URL=
CAPTCHA_LLM_API_KEY=sk-...
# Vision-capable model id (e.g. claude-opus-4-8, gpt-4o, gemini-2.5-pro)
CAPTCHA_LLM_MODEL=claude-opus-4-8
# Optional separate audio/transcription model (e.g. whisper-1). Empty = use MODEL.
CAPTCHA_LLM_AUDIO_MODEL=
CAPTCHA_LLM_TEMPERATURE=0
CAPTCHA_LLM_MAX_TOKENS=1024
CAPTCHA_LLM_TIMEOUT=60s
CAPTCHA_LOG_LEVEL=info
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .golangci.yml Makefile .env.example
git commit -m "chore: scaffold captcha-solver-mcp module and tooling"
```

---

## Phase 1 — Config

### Task 1: Env config parse + validate

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "valid anthropic",
			env: map[string]string{
				"CAPTCHA_LLM_PROVIDER": "anthropic",
				"CAPTCHA_LLM_API_KEY":  "k",
				"CAPTCHA_LLM_MODEL":    "claude-opus-4-8",
			},
			want: Config{
				Provider: "anthropic", APIKey: "k", Model: "claude-opus-4-8",
				LogLevel: "info", MaxTokens: 1024, Timeout: 60 * time.Second,
			},
		},
		{
			name:    "missing model",
			env:     map[string]string{"CAPTCHA_LLM_PROVIDER": "openai", "CAPTCHA_LLM_API_KEY": "k"},
			wantErr: true,
		},
		{
			name:    "bad provider",
			env:     map[string]string{"CAPTCHA_LLM_PROVIDER": "nope", "CAPTCHA_LLM_API_KEY": "k", "CAPTCHA_LLM_MODEL": "m"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/...`
Expected: FAIL — `undefined: Load` / `undefined: Config`.

- [ ] **Step 3: Write the implementation**

```go
// Package config loads and validates the MCP server configuration from env vars.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the validated runtime configuration. Fields are ordered
// largest-alignment-first to satisfy the fieldalignment linter.
type Config struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	AudioModel  string
	LogLevel    string
	Temperature float64
	Timeout     time.Duration
	MaxTokens   int
}

var validProviders = map[string]bool{
	"anthropic": true, "openai": true, "openai-compatible": true, "gemini": true,
}

// Load reads CAPTCHA_LLM_* env vars and returns a validated Config.
func Load() (Config, error) {
	c := Config{
		Provider:   os.Getenv("CAPTCHA_LLM_PROVIDER"),
		BaseURL:    os.Getenv("CAPTCHA_LLM_BASE_URL"),
		APIKey:     os.Getenv("CAPTCHA_LLM_API_KEY"),
		Model:      os.Getenv("CAPTCHA_LLM_MODEL"),
		AudioModel: os.Getenv("CAPTCHA_LLM_AUDIO_MODEL"),
		LogLevel:   envOr("CAPTCHA_LOG_LEVEL", "info"),
		MaxTokens:  1024,
		Timeout:    60 * time.Second,
	}
	if !validProviders[c.Provider] {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_PROVIDER must be one of anthropic|openai|openai-compatible|gemini, got %q", c.Provider)
	}
	if c.APIKey == "" && c.BaseURL == "" {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_API_KEY is required (or set CAPTCHA_LLM_BASE_URL for a keyless local endpoint)")
	}
	if c.Model == "" {
		return Config{}, fmt.Errorf("CAPTCHA_LLM_MODEL is required")
	}
	if v := os.Getenv("CAPTCHA_LLM_TEMPERATURE"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("CAPTCHA_LLM_TEMPERATURE: %w", err)
		}
		c.Temperature = f
	}
	if v := os.Getenv("CAPTCHA_LLM_MAX_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("CAPTCHA_LLM_MAX_TOKENS: %w", err)
		}
		c.MaxTokens = n
	}
	if v := os.Getenv("CAPTCHA_LLM_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("CAPTCHA_LLM_TIMEOUT: %w", err)
		}
		c.Timeout = d
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS. (Note: `t.Setenv` isolates env per subtest.)

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): env-driven config with validation"
```

---

## Phase 2 — LLM provider layer

### Task 2: Client interface + fake double

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/fake.go`
- Test: `internal/llm/fake_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/...`
Expected: FAIL — `undefined: Fake` / `undefined: Image`.

- [ ] **Step 3: Write `client.go`**

```go
// Package llm defines a provider-agnostic LLM client for vision and audio
// transcription, with concrete HTTP implementations per provider.
package llm

import "context"

// Image is a decoded image to send to a vision model.
type Image struct {
	MediaType string // e.g. "image/png", "image/jpeg"
	Data      []byte
}

// Options are per-call overrides. Zero values mean "use the client default".
type Options struct {
	Model       string
	Language    string
	Temperature float64
	MaxTokens   int
}

// Client is the abstraction every provider implements and every handler depends on.
type Client interface {
	// Vision sends a prompt plus images and returns the model's text answer.
	Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error)
	// Transcribe converts audio bytes to text. filename carries the extension
	// for providers that infer the audio format from it. Returns ErrUnsupported
	// if the provider has no audio capability.
	Transcribe(ctx context.Context, audio []byte, filename string, opts Options) (string, error)
}

// ErrUnsupported is returned by Transcribe on providers without audio support.
type ErrUnsupported struct{ Provider string }

func (e ErrUnsupported) Error() string {
	return "audio transcription unsupported by provider " + e.Provider
}
```

- [ ] **Step 4: Write `fake.go`**

```go
package llm

import "context"

// Fake is a deterministic Client for tests.
type Fake struct {
	VisionResp     string
	TranscribeResp string
	VisionErr      error
	TranscribeErr  error
	VisionCalls    []VisionCall
	TranscribeCalls []TranscribeCall
}

// VisionCall records a Vision invocation.
type VisionCall struct {
	Prompt string
	Opts   Options
	Images []Image
}

// TranscribeCall records a Transcribe invocation.
type TranscribeCall struct {
	Filename string
	Opts     Options
	Audio    []byte
}

// Vision implements Client.
func (f *Fake) Vision(_ context.Context, prompt string, images []Image, opts Options) (string, error) {
	f.VisionCalls = append(f.VisionCalls, VisionCall{Prompt: prompt, Images: images, Opts: opts})
	return f.VisionResp, f.VisionErr
}

// Transcribe implements Client.
func (f *Fake) Transcribe(_ context.Context, audio []byte, filename string, opts Options) (string, error) {
	f.TranscribeCalls = append(f.TranscribeCalls, TranscribeCall{Audio: audio, Filename: filename, Opts: opts})
	return f.TranscribeResp, f.TranscribeErr
}
```

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/llm/...` → PASS.
```bash
git add internal/llm
git commit -m "feat(llm): client interface and fake test double"
```

### Task 3: OpenAI-compatible provider (vision + transcription)

**Files:**
- Create: `internal/llm/openai.go`
- Test: `internal/llm/openai_test.go`

- [ ] **Step 1: Write the failing test** (uses `httptest`, no network)

```go
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

func TestOpenAITranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/llm/ -run OpenAI`
Expected: FAIL — `undefined: openAIClient`.

- [ ] **Step 3: Write `openai.go`**

```go
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

type openAIClient struct {
	http       *http.Client
	base       string
	key        string
	model      string
	audioModel string
	maxTokens  int
}

func (c *openAIClient) modelOr(o Options) string {
	if o.Model != "" {
		return o.Model
	}
	return c.model
}

func (c *openAIClient) Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error) {
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, img := range images {
		url := fmt.Sprintf("data:%s;base64,%s", img.MediaType, base64.StdEncoding.EncodeToString(img.Data))
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
	}
	maxTok := c.maxTokens
	if opts.MaxTokens > 0 {
		maxTok = opts.MaxTokens
	}
	reqBody := map[string]any{
		"model":       c.modelOr(opts),
		"max_tokens":  maxTok,
		"temperature": opts.Temperature,
		"messages":    []map[string]any{{"role": "user", "content": content}},
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.postJSON(ctx, "/v1/chat/completions", reqBody, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: empty choices")
	}
	return out.Choices[0].Message.Content, nil
}

func (c *openAIClient) Transcribe(ctx context.Context, audio []byte, filename string, opts Options) (string, error) {
	model := c.audioModel
	if opts.Model != "" {
		model = opts.Model
	}
	if model == "" {
		model = c.model
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", model)
	if opts.Language != "" {
		_ = mw.WriteField("language", opts.Language)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai transcribe HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

func (c *openAIClient) postJSON(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return json.Unmarshal(raw, out)
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/llm/ -run OpenAI` → PASS.
```bash
git add internal/llm/openai.go internal/llm/openai_test.go
git commit -m "feat(llm): openai-compatible vision and transcription provider"
```

### Task 4: Anthropic provider (vision; transcription unsupported)

**Files:**
- Create: `internal/llm/anthropic.go`
- Test: `internal/llm/anthropic_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/llm/ -run Anthropic`
Expected: FAIL — `undefined: anthropicClient`.

- [ ] **Step 3: Write `anthropic.go`**

```go
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type anthropicClient struct {
	http      *http.Client
	base      string
	key       string
	model     string
	maxTokens int
}

func (c *anthropicClient) Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error) {
	content := []map[string]any{}
	for _, img := range images {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": img.MediaType,
				"data":       base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	content = append(content, map[string]any{"type": "text", "text": prompt})

	model := c.model
	if opts.Model != "" {
		model = opts.Model
	}
	maxTok := c.maxTokens
	if opts.MaxTokens > 0 {
		maxTok = opts.MaxTokens
	}
	// NOTE: temperature/top_p are intentionally omitted — Opus 4.x rejects them (HTTP 400).
	reqBody := map[string]any{
		"model":      model,
		"max_tokens": maxTok,
		"messages":   []map[string]any{{"role": "user", "content": content}},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/messages", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	for _, blk := range out.Content {
		if blk.Type == "text" {
			return blk.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic: no text block in response")
}

func (c *anthropicClient) Transcribe(_ context.Context, _ []byte, _ string, _ Options) (string, error) {
	return "", ErrUnsupported{Provider: "anthropic"}
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/llm/ -run Anthropic` → PASS.
```bash
git add internal/llm/anthropic.go internal/llm/anthropic_test.go
git commit -m "feat(llm): anthropic vision provider (audio unsupported)"
```

### Task 5: Provider factory `New`

**Files:**
- Create: `internal/llm/new.go`
- Test: `internal/llm/new_test.go`

- [ ] **Step 1: Write the failing test**

```go
package llm

import (
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantBase string
		wantErr  bool
	}{
		{"anthropic default base", "anthropic", "https://api.anthropic.com", false},
		{"openai default base", "openai", "https://api.openai.com", false},
		{"compatible needs base", "openai-compatible", "", true},
		{"unknown", "zzz", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{Provider: tc.provider, APIKey: "k", Model: "m"}
			c, err := New(cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if c == nil {
				t.Fatal("nil client")
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/llm/ -run TestNew`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write `new.go`**

```go
package llm

import (
	"fmt"
	"net/http"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/config"
)

// New constructs the configured provider Client.
func New(cfg config.Config) (Client, error) {
	hc := &http.Client{Timeout: cfg.Timeout}
	switch cfg.Provider {
	case "anthropic":
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.anthropic.com"
		}
		return &anthropicClient{http: hc, base: base, key: cfg.APIKey, model: cfg.Model, maxTokens: cfg.MaxTokens}, nil
	case "openai":
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.openai.com"
		}
		return &openAIClient{http: hc, base: base, key: cfg.APIKey, model: cfg.Model, audioModel: cfg.AudioModel, maxTokens: cfg.MaxTokens}, nil
	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible requires CAPTCHA_LLM_BASE_URL")
		}
		return &openAIClient{http: hc, base: cfg.BaseURL, key: cfg.APIKey, model: cfg.Model, audioModel: cfg.AudioModel, maxTokens: cfg.MaxTokens}, nil
	case "gemini":
		base := cfg.BaseURL
		if base == "" {
			base = "https://generativelanguage.googleapis.com"
		}
		return &geminiClient{http: hc, base: base, key: cfg.APIKey, model: cfg.Model, audioModel: cfg.AudioModel, maxTokens: cfg.MaxTokens}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
```

> NOTE: `geminiClient` is implemented in Phase 11 (Task 19). Until then, comment out the `gemini` case or stub `geminiClient` so the package compiles. The cleanest order is to implement Task 19's `gemini.go` stub (struct + two methods returning `fmt.Errorf("not implemented")`) **before** this task, then fill it in Phase 11. To keep this task self-contained, add the stub now:

`internal/llm/gemini.go` (stub — replaced in Task 19):
```go
package llm

import (
	"context"
	"fmt"
	"net/http"
)

type geminiClient struct {
	http       *http.Client
	base       string
	key        string
	model      string
	audioModel string
	maxTokens  int
}

func (c *geminiClient) Vision(_ context.Context, _ string, _ []Image, _ Options) (string, error) {
	return "", fmt.Errorf("gemini provider not yet implemented")
}

func (c *geminiClient) Transcribe(_ context.Context, _ []byte, _ string, _ Options) (string, error) {
	return "", fmt.Errorf("gemini provider not yet implemented")
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/llm/...` → PASS.
```bash
git add internal/llm/new.go internal/llm/new_test.go internal/llm/gemini.go
git commit -m "feat(llm): provider factory with per-provider default base URLs"
```

---

## Phase 3 — Image utilities

### Task 6: Input decoding + grid geometry

**Files:**
- Create: `internal/imageutil/imageutil.go`
- Test: `internal/imageutil/imageutil_test.go`

- [ ] **Step 1: Write the failing test**

```go
package imageutil

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestDecode(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4e, 0x47} // PNG magic prefix
	b64 := base64.StdEncoding.EncodeToString(raw)

	t.Run("data uri", func(t *testing.T) {
		data, mt, err := Decode("data:image/png;base64," + b64)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
	t.Run("bare base64 sniffs png", func(t *testing.T) {
		data, mt, err := Decode(b64)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
	t.Run("file path", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "x.png")
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		data, mt, err := Decode(p)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
}

func TestGridCentroids(t *testing.T) {
	pts := GridCentroids(300, 300, 3, 3)
	if len(pts) != 9 {
		t.Fatalf("want 9 centroids, got %d", len(pts))
	}
	// index 0 = top-left tile centre at (50,50)
	if pts[0].X != 50 || pts[0].Y != 50 {
		t.Fatalf("centroid[0]=%+v", pts[0])
	}
	// index 4 = centre tile at (150,150)
	if pts[4].X != 150 || pts[4].Y != 150 {
		t.Fatalf("centroid[4]=%+v", pts[4])
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/imageutil/...`
Expected: FAIL — `undefined: Decode` / `undefined: GridCentroids`.

- [ ] **Step 3: Write `imageutil.go`**

```go
// Package imageutil decodes image/audio inputs (base64, data-URI, file path,
// http(s) URL) and computes grid geometry for tile-based captcha solving.
package imageutil

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var dataURIRe = regexp.MustCompile(`^data:([^;,]+)(;base64)?,(.*)$`)

// Point is a pixel coordinate.
type Point struct {
	X int
	Y int
}

// Decode resolves input (data-URI | bare base64 | file path | http(s) URL) to
// raw bytes plus a best-effort media type sniffed from the content.
func Decode(input string) ([]byte, string, error) {
	input = strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(input, "data:"):
		m := dataURIRe.FindStringSubmatch(input)
		if m == nil {
			return nil, "", fmt.Errorf("malformed data URI")
		}
		var data []byte
		var err error
		if m[2] == ";base64" {
			data, err = base64.StdEncoding.DecodeString(m[3])
		} else {
			data = []byte(m[3])
		}
		if err != nil {
			return nil, "", err
		}
		return data, m[1], nil
	case strings.HasPrefix(input, "http://"), strings.HasPrefix(input, "https://"):
		return fetchURL(input)
	default:
		// File path if it exists; otherwise treat as bare base64.
		if data, err := os.ReadFile(input); err == nil { //nolint:gosec // path is caller-provided automation input
			return data, sniff(data), nil
		}
		data, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return nil, "", fmt.Errorf("input is not a data-URI, URL, readable file, or base64: %w", err)
		}
		return data, sniff(data), nil
	}
}

func fetchURL(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	mt := resp.Header.Get("Content-Type")
	if mt == "" {
		mt = sniff(data)
	}
	return data, mt, nil
}

// sniff returns a media type from magic bytes, defaulting to image/png.
func sniff(b []byte) string {
	switch {
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4e && b[3] == 0x47:
		return "image/png"
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return "image/jpeg"
	case len(b) >= 4 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F':
		return "audio/wav"
	case len(b) >= 6 && string(b[:3]) == "ID3":
		return "audio/mpeg"
	default:
		return "image/png"
	}
}

// GridCentroids returns the pixel centre of each tile for a rows x cols grid
// laid over a width x height image, in row-major order (index 0 = top-left).
func GridCentroids(width, height, rows, cols int) []Point {
	pts := make([]Point, 0, rows*cols)
	cw := float64(width) / float64(cols)
	ch := float64(height) / float64(rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			pts = append(pts, Point{
				X: int(cw*float64(c) + cw/2),
				Y: int(ch*float64(r) + ch/2),
			})
		}
	}
	return pts
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/imageutil/...` → PASS.
```bash
git add internal/imageutil
git commit -m "feat(imageutil): input decoding and grid centroid geometry"
```

---

## Phase 4 — Captcha types + detection

### Task 7: Shared types + deterministic detection

**Files:**
- Create: `internal/captcha/types.go`
- Create: `internal/captcha/detect.go`
- Test: `internal/captcha/detect_test.go`

- [ ] **Step 1: Write the failing test**

```go
package captcha

import "testing"

func TestDetectHTML(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		wantType    string
		wantSolvable bool
	}{
		{"recaptcha v2", `<div class="g-recaptcha" data-sitekey="6Lc_abc"></div>`, "recaptcha_v2", true},
		{"recaptcha v3", `<script src="https://www.google.com/recaptcha/api.js?render=6Lc_v3key"></script>`, "recaptcha_v3", false},
		{"hcaptcha", `<div class="h-captcha" data-sitekey="hk_123"></div>`, "hcaptcha", true},
		{"turnstile", `<div class="cf-turnstile" data-sitekey="0x4AAA"></div>`, "turnstile", false},
		{"funcaptcha", `<div id="arkose" data-pkey="ABC"></div>`, "funcaptcha", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectHTML(tc.html)
			if len(got) == 0 {
				t.Fatalf("no detection")
			}
			if got[0].Type != tc.wantType || got[0].LLMSolvable != tc.wantSolvable {
				t.Fatalf("got %+v", got[0])
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/captcha/ -run TestDetectHTML`
Expected: FAIL — `undefined: DetectHTML`.

- [ ] **Step 3: Write `types.go`**

```go
// Package captcha provides deterministic detection and LLM-backed solving of
// captcha challenges. It never drives a browser and never mints tokens.
package captcha

// Detection describes one captcha found on a page. Fields are ordered
// largest-alignment-first for fieldalignment.
type Detection struct {
	Type        string `json:"type"`
	Sitekey     string `json:"sitekey,omitempty"`
	IframeURL   string `json:"iframe_url,omitempty"`
	Notes       string `json:"notes,omitempty"`
	LLMSolvable bool   `json:"llm_solvable"`
}
```

- [ ] **Step 4: Write `detect.go`**

```go
package captcha

import "regexp"

var (
	reRecaptchaV3 = regexp.MustCompile(`recaptcha/api\.js\?render=([\w-]+)`)
	reSitekey     = regexp.MustCompile(`data-sitekey="([^"]+)"`)
	rePkey        = regexp.MustCompile(`data-pkey="([^"]+)"`)
)

// DetectHTML scans raw page HTML and returns deterministic detections for known
// captcha providers. v3/Turnstile are reported with LLMSolvable=false.
func DetectHTML(html string) []Detection {
	var out []Detection
	// reCAPTCHA v3 first (script render param). v3 has no visual challenge.
	if m := reRecaptchaV3.FindStringSubmatch(html); m != nil {
		out = append(out, Detection{
			Type: "recaptcha_v3", Sitekey: m[1], LLMSolvable: false,
			Notes: "score/behavioral — not LLM-solvable; needs browser reputation",
		})
	}
	if contains(html, "g-recaptcha") || contains(html, "google.com/recaptcha/api2") {
		out = append(out, Detection{Type: "recaptcha_v2", Sitekey: firstSitekey(html), LLMSolvable: true,
			Notes: "image-grid; LLM returns tiles, caller clicks + harvests token"})
	}
	if contains(html, "h-captcha") || contains(html, "hcaptcha.com") {
		out = append(out, Detection{Type: "hcaptcha", Sitekey: firstSitekey(html), LLMSolvable: true,
			Notes: "image-grid; LLM returns tiles, caller clicks + harvests token"})
	}
	if contains(html, "cf-turnstile") || contains(html, "challenges.cloudflare.com") {
		out = append(out, Detection{Type: "turnstile", Sitekey: firstSitekey(html), LLMSolvable: false,
			Notes: "proof-of-work/behavioral — not LLM-solvable"})
	}
	if contains(html, "arkoselabs") || contains(html, "funcaptcha") || contains(html, "arkose") {
		out = append(out, Detection{Type: "funcaptcha", Sitekey: firstPkey(html), LLMSolvable: true,
			Notes: "rotation/visual — best-effort"})
	}
	return out
}

func firstSitekey(html string) string {
	if m := reSitekey.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

func firstPkey(html string) string {
	if m := rePkey.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

> Simplification note: `contains`/`indexOf` reimplement `strings.Contains`. Use `strings.Contains` directly instead — the helpers above are shown only to keep the file dependency-free if desired. Prefer `strings.Contains(html, sub)` and delete `contains`/`indexOf`. (Reviewer: pick one; `strings.Contains` is the boring choice.)

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/captcha/ -run TestDetectHTML` → PASS.
```bash
git add internal/captcha/types.go internal/captcha/detect.go internal/captcha/detect_test.go
git commit -m "feat(captcha): deterministic HTML detection for known providers"
```

---

## Phase 5 — Solve tools (LLM-backed)

These functions take an `llm.Client` and return structured results. Each handler prompt is strict and asks for JSON we parse; on parse failure we retry once with a stricter instruction, then error (never fabricate). Results carry `Confidence`.

### Task 8: Text-image OCR solve

**Files:**
- Create: `internal/captcha/text.go`
- Test: `internal/captcha/text_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/captcha/ -run SolveText`
Expected: FAIL — `undefined: SolveText`.

- [ ] **Step 3: Write `text.go`**

```go
package captcha

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// TextOptions tunes text-image OCR.
type TextOptions struct {
	Hint          string
	Charset       string
	Length        int
	CaseSensitive bool
}

// TextResult is the OCR answer.
type TextResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

func textPrompt(o TextOptions, strict bool) string {
	p := "You are solving an image text captcha. Read the exact characters shown. "
	if o.Charset != "" {
		p += "Allowed characters: " + o.Charset + ". "
	}
	if o.Length > 0 {
		p += fmt.Sprintf("Expected length: %d. ", o.Length)
	}
	if !o.CaseSensitive {
		p += "Case-insensitive. "
	}
	if o.Hint != "" {
		p += "Hint: " + o.Hint + ". "
	}
	p += `Respond ONLY with compact JSON: {"text":"<chars>","confidence":<0..1>}.`
	if strict {
		p += " You MUST output valid JSON and nothing else. Do not explain."
	}
	return p
}

// SolveText runs OCR via the vision model with one stricter retry on parse failure.
func SolveText(ctx context.Context, c llm.Client, img []byte, mediaType string, o TextOptions) (TextResult, error) {
	images := []llm.Image{{MediaType: mediaType, Data: img}}
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := c.Vision(ctx, textPrompt(o, attempt > 0), images, llm.Options{})
		if err != nil {
			return TextResult{}, err
		}
		var res TextResult
		if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err == nil && res.Text != "" {
			return res, nil
		}
	}
	return TextResult{}, fmt.Errorf("model did not return parseable captcha text")
}
```

- [ ] **Step 4: Add `extractJSON` helper**

Create `internal/captcha/json.go`:
```go
package captcha

import "strings"

// extractJSON returns the first {...} JSON object found in s, or s unchanged.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
```

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/captcha/ -run SolveText` → PASS.
```bash
git add internal/captcha/text.go internal/captcha/json.go internal/captcha/text_test.go
git commit -m "feat(captcha): LLM text-image OCR solver with strict-retry"
```

### Task 9: Audio solve

**Files:**
- Create: `internal/captcha/audio.go`
- Test: `internal/captcha/audio_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail** → `undefined: SolveAudio`.

- [ ] **Step 3: Write `audio.go`**

```go
package captcha

import (
	"context"
	"strings"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// AudioResult is the transcription answer.
type AudioResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// SolveAudio transcribes an audio captcha. Confidence is fixed at 1.0 for
// transcription providers (which do not return token-level confidence here);
// the raw transcript is trimmed.
func SolveAudio(ctx context.Context, c llm.Client, audio []byte, filename, language string) (AudioResult, error) {
	txt, err := c.Transcribe(ctx, audio, filename, llm.Options{Language: language})
	if err != nil {
		return AudioResult{}, err
	}
	return AudioResult{Text: strings.TrimSpace(txt), Confidence: 1.0}, nil
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/captcha/ -run SolveAudio` → PASS.
```bash
git add internal/captcha/audio.go internal/captcha/audio_test.go
git commit -m "feat(captcha): audio captcha transcription solver"
```

### Task 10: Grid solve (reCAPTCHA v2 / hCaptcha)

**Files:**
- Create: `internal/captcha/grid.go`
- Test: `internal/captcha/grid_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail** → `undefined: SolveGrid`.

- [ ] **Step 3: Write `grid.go`**

```go
package captcha

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/imageutil"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// GridOptions describes a reCAPTCHA v2 / hCaptcha tile grid challenge.
type GridOptions struct {
	Instruction string
	CaptchaType string // recaptcha | hcaptcha (informational)
	Rows        int
	Cols        int
	ImageWidth  int
	ImageHeight int
}

// GridResult lists tiles to click (row-major 0-based) plus optional pixel
// centroids when grid dimensions are known.
type GridResult struct {
	Tiles              []int            `json:"tiles"`
	Centroids          []imageutil.Point `json:"centroids,omitempty"`
	Confidence         float64          `json:"confidence"`
	RecheckRecommended bool             `json:"recheck_recommended"`
}

func gridPrompt(o GridOptions, strict bool) string {
	p := fmt.Sprintf(
		"You are solving an image-grid captcha. The image is a %dx%d grid of tiles, "+
			"numbered row-major starting at 0 (top-left). Instruction: %q. "+
			"Return the 0-based indices of every tile that matches the instruction. "+
			"If after selection more matching tiles would likely appear (dynamic grid), set recheck_recommended true. "+
			`Respond ONLY with compact JSON: {"tiles":[..],"confidence":<0..1>,"recheck_recommended":<bool>}.`,
		o.Rows, o.Cols, o.Instruction)
	if strict {
		p += " You MUST output valid JSON only. No prose."
	}
	return p
}

// SolveGrid asks the vision model which tiles to click and (when grid geometry
// is supplied) maps them to pixel centroids for humanized clicking.
func SolveGrid(ctx context.Context, c llm.Client, img []byte, mediaType string, o GridOptions) (GridResult, error) {
	images := []llm.Image{{MediaType: mediaType, Data: img}}
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := c.Vision(ctx, gridPrompt(o, attempt > 0), images, llm.Options{})
		if err != nil {
			return GridResult{}, err
		}
		var res GridResult
		if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err == nil && res.Tiles != nil {
			res.Centroids = mapCentroids(res.Tiles, o)
			return res, nil
		}
	}
	return GridResult{}, fmt.Errorf("model did not return parseable tile selection")
}

func mapCentroids(tiles []int, o GridOptions) []imageutil.Point {
	if o.Rows <= 0 || o.Cols <= 0 || o.ImageWidth <= 0 || o.ImageHeight <= 0 {
		return nil
	}
	all := imageutil.GridCentroids(o.ImageWidth, o.ImageHeight, o.Rows, o.Cols)
	out := make([]imageutil.Point, 0, len(tiles))
	for _, idx := range tiles {
		if idx >= 0 && idx < len(all) {
			out = append(out, all[idx])
		}
	}
	return out
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/captcha/ -run SolveGrid` → PASS.
```bash
git add internal/captcha/grid.go internal/captcha/grid_test.go
git commit -m "feat(captcha): image-grid tile solver with centroid mapping"
```

### Task 11: Rotation/FunCaptcha solve (best-effort)

**Files:**
- Create: `internal/captcha/rotation.go`
- Test: `internal/captcha/rotation_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail** → `undefined: SolveRotation`.

- [ ] **Step 3: Write `rotation.go`**

```go
package captcha

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// RotationResult is a best-effort FunCaptcha/rotation answer.
type RotationResult struct {
	Choice     int     `json:"choice"`
	Rotations  int     `json:"rotations"`
	Confidence float64 `json:"confidence"`
}

// SolveRotation handles FunCaptcha-style rotation/choice puzzles (best-effort).
func SolveRotation(ctx context.Context, c llm.Client, img []byte, mediaType, instruction string) (RotationResult, error) {
	prompt := fmt.Sprintf(
		"You are solving a rotation/choice puzzle captcha. Instruction: %q. "+
			"If asked to pick one image, return its 0-based index as choice. "+
			"If asked to rotate, return the number of clockwise steps as rotations. "+
			`Respond ONLY with compact JSON: {"choice":<int>,"rotations":<int>,"confidence":<0..1>}.`,
		instruction)
	raw, err := c.Vision(ctx, prompt, []llm.Image{{MediaType: mediaType, Data: img}}, llm.Options{})
	if err != nil {
		return RotationResult{}, err
	}
	var res RotationResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return RotationResult{}, fmt.Errorf("model did not return parseable rotation answer")
	}
	return res, nil
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/captcha/ -run SolveRotation` → PASS.
```bash
git add internal/captcha/rotation.go internal/captcha/rotation_test.go
git commit -m "feat(captcha): best-effort rotation/funcaptcha solver"
```

---

## Phase 6 — Stealth subsystem

### Task 12: Fingerprint generator (coherent, seeded)

**Files:**
- Create: `internal/stealth/fingerprint.go`
- Test: `internal/stealth/fingerprint_test.go`

- [ ] **Step 1: Write the failing test**

```go
package stealth

import "testing"

func TestGenerateFingerprintDeterministic(t *testing.T) {
	a := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Locale: "en-US", Seed: 42})
	b := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Locale: "en-US", Seed: 42})
	if a != b {
		t.Fatalf("same seed must be deterministic:\n%+v\n%+v", a, b)
	}
	if a.UserAgent == "" || a.Platform == "" || a.SecCHUA == "" {
		t.Fatalf("incomplete fingerprint: %+v", a)
	}
	// internal consistency: windows UA must mention Windows and platform Win32
	if a.Platform != "Win32" {
		t.Fatalf("windows platform=%q", a.Platform)
	}
}

func TestGenerateFingerprintVaries(t *testing.T) {
	a := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Seed: 1})
	b := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Seed: 2})
	if a.HardwareConcurrency == b.HardwareConcurrency && a.UserAgent == b.UserAgent && a.Viewport == b.Viewport {
		t.Fatal("different seeds should vary at least one attribute")
	}
}
```

- [ ] **Step 2: Run to verify fail** → `undefined: GenerateFingerprint`.

- [ ] **Step 3: Write `fingerprint.go`** (deterministic PRNG seeded by `Seed`; no `math/rand` global, no `Date.now`)

```go
// Package stealth generates browser anti-bot artifacts (fingerprint profiles,
// evasion init scripts, humanized interaction plans, header sets) that the
// caller injects/applies. It never drives a browser.
package stealth

import (
	"fmt"
	"math/rand/v2"
)

// FingerprintParams selects an identity. Seed makes generation deterministic.
type FingerprintParams struct {
	OS      string // windows | macos | linux
	Browser string // chrome | firefox
	Locale  string // e.g. en-US
	Seed    uint64
}

// Fingerprint is a coherent, internally-consistent browser identity.
type Fingerprint struct {
	UserAgent          string `json:"user_agent"`
	SecCHUA            string `json:"sec_ch_ua"`
	SecCHUAPlatform    string `json:"sec_ch_ua_platform"`
	Platform           string `json:"platform"`
	Locale             string `json:"locale"`
	Timezone           string `json:"timezone"`
	Viewport           string `json:"viewport"`
	WebGLVendor        string `json:"webgl_vendor"`
	WebGLRenderer      string `json:"webgl_renderer"`
	HardwareConcurrency int   `json:"hardware_concurrency"`
	DeviceMemory       int    `json:"device_memory"`
}

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

// GenerateFingerprint returns a coherent identity. Same Seed → same output.
func GenerateFingerprint(p FingerprintParams) Fingerprint {
	if p.OS == "" {
		p.OS = "windows"
	}
	if p.Browser == "" {
		p.Browser = "chrome"
	}
	if p.Locale == "" {
		p.Locale = "en-US"
	}
	r := rand.New(rand.NewPCG(p.Seed, p.Seed^0x9e3779b97f4a7c15))

	chromeMajor := pick(r, []string{"131", "132", "133"})
	var ua, platform, chPlatform, webglVendor, webglRenderer, tz string
	switch p.OS {
	case "macos":
		platform, chPlatform = "MacIntel", `"macOS"`
		ua = fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (Apple)", "ANGLE (Apple, Apple M2, OpenGL 4.1)"
		tz = "America/Los_Angeles"
	case "linux":
		platform, chPlatform = "Linux x86_64", `"Linux"`
		ua = fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (Mesa)", "ANGLE (Mesa, llvmpipe, OpenGL 4.5)"
		tz = "Europe/London"
	default: // windows
		platform, chPlatform = "Win32", `"Windows"`
		ua = fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11, D3D11)"
		tz = "America/New_York"
	}
	secCHUA := fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not?A_Brand";v="24"`, chromeMajor, chromeMajor)
	viewport := pick(r, []string{"1920x1080", "1536x864", "1440x900", "1366x768"})
	return Fingerprint{
		UserAgent:           ua,
		SecCHUA:             secCHUA,
		SecCHUAPlatform:     chPlatform,
		Platform:            platform,
		Locale:              p.Locale,
		Timezone:            tz,
		Viewport:            viewport,
		WebGLVendor:         webglVendor,
		WebGLRenderer:       webglRenderer,
		HardwareConcurrency: pick(r, []int{4, 8, 12, 16}),
		DeviceMemory:        pick(r, []int{4, 8, 16}),
	}
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/stealth/ -run Fingerprint` → PASS.
```bash
git add internal/stealth/fingerprint.go internal/stealth/fingerprint_test.go
git commit -m "feat(stealth): deterministic coherent fingerprint generator"
```

### Task 13: Evasion init script

**Files:**
- Create: `internal/stealth/script.go`
- Create: `internal/stealth/assets/evasions.js`
- Test: `internal/stealth/script_test.go`

- [ ] **Step 1: Write the failing test**

```go
package stealth

import "testing"

func TestStealthScript(t *testing.T) {
	res := StealthScript(ScriptParams{Engine: "playwright"})
	if res.Script == "" {
		t.Fatal("empty script")
	}
	for _, want := range []string{"navigator.webdriver", "languages", "WebGL", "chrome"} {
		if !contains(res.Script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
	if res.Apply.How == "" || len(res.IncludedEvasions) == 0 {
		t.Fatalf("missing apply metadata: %+v", res)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run to verify fail** → `undefined: StealthScript`.

- [ ] **Step 3: Write `assets/evasions.js`**

> The init script patches the common bot-detection surfaces. This is a minimal, self-contained version (no external library). At implementation, cross-check against current open-source evasion sets (e.g. playwright-stealth / puppeteer-extra-plugin-stealth) and expand; record the source + license in `internal/stealth/assets/NOTICE`.

```javascript
(() => {
  // navigator.webdriver
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  // languages
  Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
  // plugins (non-empty)
  Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
  // chrome runtime stub
  window.chrome = window.chrome || { runtime: {} };
  // permissions query consistency
  const origQuery = window.navigator.permissions && window.navigator.permissions.query;
  if (origQuery) {
    window.navigator.permissions.query = (params) =>
      params && params.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : origQuery(params);
  }
  // WebGL vendor/renderer spoof
  const getParam = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function (p) {
    if (p === 37445) return 'Google Inc.';        // UNMASKED_VENDOR_WEBGL
    if (p === 37446) return 'ANGLE (Generic GPU)'; // UNMASKED_RENDERER_WEBGL
    return getParam.call(this, p);
  };
})();
```

- [ ] **Step 4: Write `script.go`** (embeds the asset)

```go
package stealth

import _ "embed"

//go:embed assets/evasions.js
var evasionsJS string

// ScriptParams selects the target engine for apply instructions.
type ScriptParams struct {
	Engine string // playwright | cdp | generic
}

// ApplyInfo tells the caller how to inject the script.
type ApplyInfo struct {
	How string `json:"how"`
	API string `json:"api"`
}

// ScriptResult is the injectable evasion bundle plus metadata.
type ScriptResult struct {
	Script           string    `json:"script"`
	Apply            ApplyInfo `json:"apply"`
	IncludedEvasions []string  `json:"included_evasions"`
}

// StealthScript returns the evasion init script and engine-specific apply hints.
func StealthScript(p ScriptParams) ScriptResult {
	apply := ApplyInfo{How: "Inject before any page script runs.", API: "page.addInitScript(script)"}
	switch p.Engine {
	case "cdp":
		apply.API = "Page.addScriptToEvaluateOnNewDocument({source: script})"
	case "generic", "":
		apply.API = "evaluate the script on every new document before navigation"
	}
	return ScriptResult{
		Script: evasionsJS,
		Apply:  apply,
		IncludedEvasions: []string{
			"navigator.webdriver", "navigator.languages", "navigator.plugins",
			"chrome.runtime", "permissions.query", "WebGL vendor/renderer",
		},
	}
}
```

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/stealth/ -run StealthScript` → PASS.
```bash
git add internal/stealth/script.go internal/stealth/assets internal/stealth/script_test.go
git commit -m "feat(stealth): embeddable evasion init script with apply hints"
```

### Task 14: Humanized interaction plan

**Files:**
- Create: `internal/stealth/interaction.go`
- Test: `internal/stealth/interaction_test.go`

- [ ] **Step 1: Write the failing test**

```go
package stealth

import "testing"

func TestPlanInteraction(t *testing.T) {
	plan := PlanInteraction(InteractionParams{
		Actions: []Action{{Type: "click", X: 50, Y: 50}, {Type: "click", X: 150, Y: 150}},
		Seed:    7,
	})
	if len(plan.Steps) == 0 {
		t.Fatal("no steps")
	}
	// monotonic non-decreasing timing
	last := -1.0
	for _, s := range plan.Steps {
		if s.AtMs < last {
			t.Fatalf("timing not monotonic: %v then %v", last, s.AtMs)
		}
		last = s.AtMs
	}
	// last mouse step must land on the final target
	var lastMove *Step
	for i := range plan.Steps {
		if plan.Steps[i].Type == "move" {
			lastMove = &plan.Steps[i]
		}
	}
	if lastMove == nil || lastMove.X != 150 || lastMove.Y != 150 {
		t.Fatalf("final move did not reach target: %+v", lastMove)
	}
}
```

- [ ] **Step 2: Run to verify fail** → `undefined: PlanInteraction`.

- [ ] **Step 3: Write `interaction.go`** (Bézier path + jitter + Fitts-law-ish timing, seeded)

```go
package stealth

import (
	"math"
	"math/rand/v2"
)

// Action is a high-level target the caller wants to perform.
type Action struct {
	Type string `json:"type"` // click | type | scroll
	Text string `json:"text,omitempty"`
	X    int    `json:"x,omitempty"`
	Y    int    `json:"y,omitempty"`
}

// InteractionParams is the input to PlanInteraction.
type InteractionParams struct {
	Actions []Action
	Seed    uint64
}

// Step is one low-level humanized event with a relative timestamp (ms).
type Step struct {
	Type string  `json:"type"` // move | down | up | key
	Key  string  `json:"key,omitempty"`
	AtMs float64 `json:"at_ms"`
	X    int     `json:"x,omitempty"`
	Y    int     `json:"y,omitempty"`
}

// InteractionPlan is a humanized timeline the caller replays.
type InteractionPlan struct {
	Steps []Step `json:"steps"`
}

// PlanInteraction converts target actions into a humanized event timeline:
// cubic-Bézier mouse paths with jitter and Fitts-law-derived durations, plus
// per-key typing cadence. Deterministic for a given Seed.
func PlanInteraction(p InteractionParams) InteractionPlan {
	r := rand.New(rand.NewPCG(p.Seed, p.Seed^0xa0761d6478bd642f))
	var steps []Step
	t := 0.0
	cx, cy := 0, 0
	for _, a := range p.Actions {
		switch a.Type {
		case "click":
			seg, dt := mousePath(r, cx, cy, a.X, a.Y, t)
			steps = append(steps, seg...)
			t += dt
			steps = append(steps, Step{Type: "down", X: a.X, Y: a.Y, AtMs: t})
			t += 40 + r.Float64()*60 // dwell
			steps = append(steps, Step{Type: "up", X: a.X, Y: a.Y, AtMs: t})
			t += 60 + r.Float64()*120
			cx, cy = a.X, a.Y
		case "type":
			for _, ch := range a.Text {
				t += 60 + r.Float64()*120 // inter-key cadence
				steps = append(steps, Step{Type: "key", Key: string(ch), AtMs: t})
			}
		}
	}
	return InteractionPlan{Steps: steps}
}

func mousePath(r *rand.Rand, x0, y0, x1, y1 int, start float64) ([]Step, float64) {
	dist := math.Hypot(float64(x1-x0), float64(y1-y0))
	// Fitts-law-ish duration; bounded.
	dur := 120 + 100*math.Log2(1+dist/40)
	n := 12 + r.IntN(8)
	// random control points around the straight line for a curved path
	c1x := float64(x0) + (float64(x1-x0))*0.3 + (r.Float64()-0.5)*dist*0.3
	c1y := float64(y0) + (float64(y1-y0))*0.3 + (r.Float64()-0.5)*dist*0.3
	c2x := float64(x0) + (float64(x1-x0))*0.7 + (r.Float64()-0.5)*dist*0.3
	c2y := float64(y0) + (float64(y1-y0))*0.7 + (r.Float64()-0.5)*dist*0.3
	steps := make([]Step, 0, n)
	for i := 1; i <= n; i++ {
		u := float64(i) / float64(n)
		bx := cubic(u, float64(x0), c1x, c2x, float64(x1))
		by := cubic(u, float64(y0), c1y, c2y, float64(y1))
		jitter := 0.0
		if i < n { // no jitter on the final landing point
			jitter = (r.Float64() - 0.5) * 2
		}
		steps = append(steps, Step{
			Type: "move",
			X:    int(bx + jitter),
			Y:    int(by + jitter),
			AtMs: start + dur*u,
		})
	}
	return steps, dur
}

func cubic(u, p0, p1, p2, p3 float64) float64 {
	v := 1 - u
	return v*v*v*p0 + 3*v*v*u*p1 + 3*v*u*u*p2 + u*u*u*p3
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/stealth/ -run PlanInteraction` → PASS.
```bash
git add internal/stealth/interaction.go internal/stealth/interaction_test.go
git commit -m "feat(stealth): humanized interaction plan (bezier + cadence)"
```

### Task 15: Header / client-hint set

**Files:**
- Create: `internal/stealth/headers.go`
- Test: `internal/stealth/headers_test.go`

- [ ] **Step 1: Write the failing test**

```go
package stealth

import "testing"

func TestHeadersFor(t *testing.T) {
	fp := GenerateFingerprint(FingerprintParams{OS: "macos", Browser: "chrome", Locale: "en-GB", Seed: 5})
	h := HeadersFor(fp)
	if h.Headers["User-Agent"] != fp.UserAgent {
		t.Fatalf("UA mismatch")
	}
	if h.Headers["Accept-Language"] == "" || h.Headers["Sec-CH-UA-Platform"] == "" {
		t.Fatalf("missing client hints: %+v", h.Headers)
	}
	if len(h.Advisories) == 0 {
		t.Fatal("expected TLS/JA3 advisory")
	}
}
```

- [ ] **Step 2: Run to verify fail** → `undefined: HeadersFor`.

- [ ] **Step 3: Write `headers.go`**

```go
package stealth

// HeaderSet is a consistent header bundle plus advisories the MCP cannot enforce.
type HeaderSet struct {
	Headers    map[string]string `json:"headers"`
	Advisories []string          `json:"advisories"`
}

// HeadersFor builds an HTTP header set consistent with the given fingerprint.
func HeadersFor(fp Fingerprint) HeaderSet {
	return HeaderSet{
		Headers: map[string]string{
			"User-Agent":         fp.UserAgent,
			"Accept-Language":    fp.Locale + ",en;q=0.9",
			"Sec-CH-UA":          fp.SecCHUA,
			"Sec-CH-UA-Platform": fp.SecCHUAPlatform,
			"Sec-CH-UA-Mobile":   "?0",
			"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		},
		Advisories: []string{
			"TLS/JA3 fingerprint is set by the browser's TLS stack and cannot be changed by this MCP — use a matching browser build or a TLS-rotating proxy.",
			"HTTP/2 frame fingerprint is browser-controlled; header order here is advisory only.",
		},
	}
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/stealth/...` → PASS.
```bash
git add internal/stealth/headers.go internal/stealth/headers_test.go
git commit -m "feat(stealth): consistent header set with TLS/JA3 advisories"
```

---

## Phase 7 — MCP server wiring

### Task 16: Tool registration + handlers

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools.go`
- Test: `internal/mcp/tools_test.go`

The handlers use the SDK's typed signature: `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`. We unit-test the handler functions directly (not over a transport) with a `llm.Fake`.

- [ ] **Step 1: Write the failing test**

```go
package mcpserver

import (
	"context"
	"testing"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

func TestGridHandler(t *testing.T) {
	h := New(&llm.Fake{VisionResp: `{"tiles":[1,2],"confidence":0.7,"recheck_recommended":false}`}, "anthropic", "claude-opus-4-8")
	_, out, err := h.solveGrid(context.Background(), nil, SolveGridIn{
		Screenshot: pngB64(), Instruction: "buses", Rows: 3, Cols: 3, ImageWidth: 300, ImageHeight: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Tiles) != 2 || len(out.Centroids) != 2 {
		t.Fatalf("out=%+v", out)
	}
}

func TestDetectHandler(t *testing.T) {
	h := New(&llm.Fake{}, "anthropic", "claude-opus-4-8")
	_, out, err := h.detect(context.Background(), nil, DetectIn{HTML: `<div class="g-recaptcha" data-sitekey="k"></div>`})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Detected) != 1 || out.Detected[0].Type != "recaptcha_v2" {
		t.Fatalf("out=%+v", out)
	}
}

// pngB64 returns a 1px PNG as base64 (valid magic bytes are enough for decode).
func pngB64() string { return "iVBORw0KGgo=" } // "iVBORw..." decodes to PNG magic prefix
```

> Note: the base64 in `pngB64` only needs to decode to bytes beginning with the PNG magic so `imageutil.Decode` sniffs `image/png`; the vision call is faked, so no real rendering happens.

- [ ] **Step 2: Run to verify fail** → `undefined: New`.

- [ ] **Step 3: Write `tools.go`** (In/Out structs + handlers)

```go
// Package mcpserver wires captcha + stealth functions to MCP tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/captcha"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/imageutil"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/stealth"
)

// Handlers holds dependencies shared by all tool handlers.
type Handlers struct {
	llm      llm.Client
	provider string
	model    string
}

// New constructs Handlers.
func New(c llm.Client, provider, model string) *Handlers {
	return &Handlers{llm: c, provider: provider, model: model}
}

// ---- detect_captcha ----

type DetectIn struct {
	HTML       string `json:"html,omitempty" jsonschema:"page HTML to scan for captcha markers"`
	URL        string `json:"url,omitempty" jsonschema:"page URL (informational)"`
	Screenshot string `json:"screenshot,omitempty" jsonschema:"base64/path/url screenshot, used only if HTML is inconclusive"`
}
type DetectOut struct {
	Detected []captcha.Detection `json:"detected"`
	Summary  string              `json:"summary"`
}

func (h *Handlers) detect(_ context.Context, _ *mcp.CallToolRequest, in DetectIn) (*mcp.CallToolResult, DetectOut, error) {
	det := captcha.DetectHTML(in.HTML)
	summary := fmt.Sprintf("%d captcha(s) detected", len(det))
	return nil, DetectOut{Detected: det, Summary: summary}, nil
}

// ---- solve_text_captcha ----

type SolveTextIn struct {
	Image         string `json:"image" jsonschema:"base64/path/url/data-uri of the text captcha image"`
	Hint          string `json:"hint,omitempty"`
	Charset       string `json:"charset,omitempty"`
	Length        int    `json:"length,omitempty"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
}

func (h *Handlers) solveText(ctx context.Context, _ *mcp.CallToolRequest, in SolveTextIn) (*mcp.CallToolResult, captcha.TextResult, error) {
	data, mt, err := imageutil.Decode(in.Image)
	if err != nil {
		return nil, captcha.TextResult{}, err
	}
	res, err := captcha.SolveText(ctx, h.llm, data, mt, captcha.TextOptions{
		Hint: in.Hint, Charset: in.Charset, Length: in.Length, CaseSensitive: in.CaseSensitive,
	})
	return nil, res, err
}

// ---- solve_audio_captcha ----

type SolveAudioIn struct {
	Audio    string `json:"audio" jsonschema:"base64/path/url of the audio captcha"`
	Language string `json:"language,omitempty"`
}

func (h *Handlers) solveAudio(ctx context.Context, _ *mcp.CallToolRequest, in SolveAudioIn) (*mcp.CallToolResult, captcha.AudioResult, error) {
	data, _, err := imageutil.Decode(in.Audio)
	if err != nil {
		return nil, captcha.AudioResult{}, err
	}
	res, err := captcha.SolveAudio(ctx, h.llm, data, "audio.wav", in.Language)
	var unsupported llm.ErrUnsupported
	if errors.As(err, &unsupported) {
		return nil, captcha.AudioResult{}, fmt.Errorf("audio captcha unsupported: configured provider %q has no transcription; set CAPTCHA_LLM_PROVIDER=openai|gemini and CAPTCHA_LLM_AUDIO_MODEL", h.provider)
	}
	return nil, res, err
}

// ---- solve_grid_captcha ----

type SolveGridIn struct {
	Screenshot         string `json:"screenshot" jsonschema:"base64/path/url of the challenge grid image"`
	Instruction        string `json:"instruction" jsonschema:"the challenge text, e.g. 'select all buses'"`
	CaptchaType        string `json:"captcha_type,omitempty" jsonschema:"recaptcha | hcaptcha"`
	Rows               int    `json:"rows" jsonschema:"grid rows (e.g. 3)"`
	Cols               int    `json:"cols" jsonschema:"grid cols (e.g. 3)"`
	ImageWidth         int    `json:"image_width,omitempty" jsonschema:"screenshot width in px, enables centroid output"`
	ImageHeight        int    `json:"image_height,omitempty" jsonschema:"screenshot height in px, enables centroid output"`
}

func (h *Handlers) solveGrid(ctx context.Context, _ *mcp.CallToolRequest, in SolveGridIn) (*mcp.CallToolResult, captcha.GridResult, error) {
	data, mt, err := imageutil.Decode(in.Screenshot)
	if err != nil {
		return nil, captcha.GridResult{}, err
	}
	res, err := captcha.SolveGrid(ctx, h.llm, data, mt, captcha.GridOptions{
		Instruction: in.Instruction, CaptchaType: in.CaptchaType,
		Rows: in.Rows, Cols: in.Cols, ImageWidth: in.ImageWidth, ImageHeight: in.ImageHeight,
	})
	return nil, res, err
}

// ---- solve_rotation_captcha ----

type SolveRotationIn struct {
	Screenshot  string `json:"screenshot" jsonschema:"base64/path/url of the rotation/choice puzzle"`
	Instruction string `json:"instruction,omitempty"`
}

func (h *Handlers) solveRotation(ctx context.Context, _ *mcp.CallToolRequest, in SolveRotationIn) (*mcp.CallToolResult, captcha.RotationResult, error) {
	data, mt, err := imageutil.Decode(in.Screenshot)
	if err != nil {
		return nil, captcha.RotationResult{}, err
	}
	res, err := captcha.SolveRotation(ctx, h.llm, data, mt, in.Instruction)
	return nil, res, err
}

// ---- stealth tools ----

type StealthScriptIn struct {
	Engine string `json:"engine,omitempty" jsonschema:"playwright | cdp | generic"`
}

func (h *Handlers) stealthScript(_ context.Context, _ *mcp.CallToolRequest, in StealthScriptIn) (*mcp.CallToolResult, stealth.ScriptResult, error) {
	return nil, stealth.StealthScript(stealth.ScriptParams{Engine: in.Engine}), nil
}

type FingerprintIn struct {
	OS      string `json:"os,omitempty" jsonschema:"windows | macos | linux"`
	Browser string `json:"browser,omitempty" jsonschema:"chrome | firefox"`
	Locale  string `json:"locale,omitempty"`
	Seed    uint64 `json:"seed,omitempty" jsonschema:"seed for a stable identity across runs"`
}
type FingerprintOut struct {
	Fingerprint stealth.Fingerprint `json:"fingerprint"`
	HeaderSet   stealth.HeaderSet   `json:"header_set"`
}

func (h *Handlers) fingerprint(_ context.Context, _ *mcp.CallToolRequest, in FingerprintIn) (*mcp.CallToolResult, FingerprintOut, error) {
	fp := stealth.GenerateFingerprint(stealth.FingerprintParams{OS: in.OS, Browser: in.Browser, Locale: in.Locale, Seed: in.Seed})
	return nil, FingerprintOut{Fingerprint: fp, HeaderSet: stealth.HeadersFor(fp)}, nil
}

type PlanInteractionIn struct {
	Actions []stealth.Action `json:"actions" jsonschema:"clicks/types/scrolls to humanize"`
	Seed    uint64           `json:"seed,omitempty"`
}

func (h *Handlers) planInteraction(_ context.Context, _ *mcp.CallToolRequest, in PlanInteractionIn) (*mcp.CallToolResult, stealth.InteractionPlan, error) {
	return nil, stealth.PlanInteraction(stealth.InteractionParams{Actions: in.Actions, Seed: in.Seed}), nil
}

// ---- solver_info ----

type SolverInfoOut struct {
	Provider     string            `json:"provider"`
	Model        string            `json:"model"`
	Capabilities map[string]bool   `json:"capabilities"`
	Notes        map[string]string `json:"notes"`
}

func (h *Handlers) solverInfo(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SolverInfoOut, error) {
	audio := h.provider == "openai" || h.provider == "openai-compatible" || h.provider == "gemini"
	return nil, SolverInfoOut{
		Provider: h.provider, Model: h.model,
		Capabilities: map[string]bool{
			"text": true, "grid": true, "rotation": true, "audio": audio, "detect": true,
			"stealth": true,
		},
		Notes: map[string]string{
			"recaptcha_v3": "detect-only, not LLM-solvable",
			"turnstile":    "detect-only, not LLM-solvable",
		},
	}, nil
}
```

- [ ] **Step 4: Write `server.go`** (registers tools on an `*mcp.Server`)

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Register attaches all tools to the given MCP server.
func (h *Handlers) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "detect_captcha", Description: "Detect captcha type/sitekey from page HTML or screenshot. Flags v3/Turnstile as not LLM-solvable."}, h.detect)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_text_captcha", Description: "OCR a distorted-text image captcha and return the characters."}, h.solveText)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_audio_captcha", Description: "Transcribe an audio captcha (requires an audio-capable provider)."}, h.solveAudio)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_grid_captcha", Description: "reCAPTCHA v2 / hCaptcha: return which grid tiles to click (and centroids). Caller clicks + harvests the token."}, h.solveGrid)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_rotation_captcha", Description: "Best-effort FunCaptcha/rotation puzzle solver."}, h.solveRotation)
	mcp.AddTool(s, &mcp.Tool{Name: "get_stealth_script", Description: "Return an evasion JS init script the caller injects, plus apply instructions."}, h.stealthScript)
	mcp.AddTool(s, &mcp.Tool{Name: "generate_fingerprint", Description: "Generate a coherent browser fingerprint + matching header set (optionally seeded)."}, h.fingerprint)
	mcp.AddTool(s, &mcp.Tool{Name: "plan_interaction", Description: "Convert target clicks/typing into a humanized mouse/keyboard timeline."}, h.planInteraction)
	mcp.AddTool(s, &mcp.Tool{Name: "solver_info", Description: "Report configured provider/model and per-type capabilities."}, h.solverInfo)
}
```

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/mcp/...` → PASS.
```bash
git add internal/mcp
git commit -m "feat(mcp): tool registration and handlers for solve+stealth+detect"
```

### Task 17: main entrypoint

**Files:**
- Create: `cmd/captcha-solver-mcp/main.go`

- [ ] **Step 1: Write `main.go`**

```go
// Command captcha-solver-mcp runs the captcha solver MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/config"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
	mcpserver "github.com/lukaszraczylo/captcha-solver-mcp/internal/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	client, err := llm.New(cfg)
	if err != nil {
		log.Fatalf("llm: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "captcha-solver-mcp", Version: "v0.1.0"}, nil)
	mcpserver.New(client, cfg.Provider, cfg.Model).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 2: Build to verify it compiles + links**

Run: `make build`
Expected: `bin/captcha-solver-mcp` produced, exit 0.

- [ ] **Step 3: Smoke-test config failure path**

Run: `CAPTCHA_LLM_PROVIDER=bad ./bin/captcha-solver-mcp`
Expected: exits non-zero with `config: CAPTCHA_LLM_PROVIDER must be one of ...`.

- [ ] **Step 4: Commit**

```bash
git add cmd/captcha-solver-mcp/main.go
git commit -m "feat(cmd): stdio entrypoint wiring config+llm+mcp"
```

---

## Phase 8 — Lint pass + full unit gate

### Task 18: golangci clean + fieldalignment

- [ ] **Step 1: Run the linter**

Run: `make lint`
Expected: zero issues. If `fieldalignment` reports a struct, fix by reordering fields (do **not** disable the linter). Re-run.

- [ ] **Step 2: Run all unit tests + vet**

Run: `go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "chore: golangci clean (fieldalignment, vet)"
```

---

## Phase 9 — Gemini provider (deferred verification)

### Task 19: Implement gemini provider

> **Grounding gate:** Before writing code, fetch the current `generateContent` request/response shape via context7 (`resolve-library-id` "Google Generative Language API" / "Gemini API", then `query-docs` for "generateContent inline_data base64 image request body; audio inline_data; response candidates parts text"). Confirm the body below matches; adjust field names if the API differs. Do not ship unverified.

**Files:**
- Replace stub: `internal/llm/gemini.go`
- Test: `internal/llm/gemini_test.go`

- [ ] **Step 1: Write the failing test** (httptest; target shape below — reconcile with verified docs)

```go
package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeminiVision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "key" {
			t.Errorf("missing key query")
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "inline_data") {
			t.Errorf("inline_data missing: %s", raw)
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"motorcycle"}]}}]}`)
	}))
	defer srv.Close()

	c := &geminiClient{http: &http.Client{Timeout: 5 * time.Second}, base: srv.URL, key: "key", model: "gemini-2.5-pro", maxTokens: 100}
	got, err := c.Vision(context.Background(), "which", []Image{{MediaType: "image/png", Data: []byte{1}}}, Options{})
	if err != nil || got != "motorcycle" {
		t.Fatalf("got %q err %v", got, err)
	}
}
```

- [ ] **Step 2: Run to verify fail** (stub returns "not implemented").

- [ ] **Step 3: Replace `gemini.go`** (reconcile with verified docs first)

```go
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type geminiClient struct {
	http       *http.Client
	base       string
	key        string
	model      string
	audioModel string
	maxTokens  int
}

func (c *geminiClient) Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error) {
	parts := []map[string]any{{"text": prompt}}
	for _, img := range images {
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": img.MediaType,
				"data":      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	model := c.model
	if opts.Model != "" {
		model = opts.Model
	}
	body := map[string]any{"contents": []map[string]any{{"parts": parts}}}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.base, model, c.key)
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty candidates")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}

func (c *geminiClient) Transcribe(ctx context.Context, audio []byte, _ string, opts Options) (string, error) {
	// Gemini transcribes via generateContent with audio inline_data.
	model := c.audioModel
	if opts.Model != "" {
		model = opts.Model
	}
	if model == "" {
		model = c.model
	}
	parts := []map[string]any{
		{"text": "Transcribe the spoken characters in this audio captcha. Output only the transcription."},
		{"inline_data": map[string]any{"mime_type": "audio/wav", "data": base64.StdEncoding.EncodeToString(audio)}},
	}
	body := map[string]any{"contents": []map[string]any{{"parts": parts}}}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.base, model, c.key)
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty candidates")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/llm/ -run Gemini && make lint` → PASS/clean.
```bash
git add internal/llm/gemini.go internal/llm/gemini_test.go
git commit -m "feat(llm): gemini vision+audio provider via generateContent"
```

---

## Phase 10 — Deterministic e2e (both engines, fake LLM, local page)

The e2e suite (build tag `e2e`) launches the **real server binary over stdio** using a minimal MCP client, points the server at a **fake LLM HTTP server**, and drives a **local fake-captcha page** with Playwright (Chromium) and Lightpanda.

### Task 20: Local fake-captcha test page

**Files:**
- Create: `test/e2e/testpage/index.html`
- Create: `test/e2e/testpage/grid.png` (a 3x3 image with one obvious row of "buses"; generated by a tiny Go gen or committed asset)

- [ ] **Step 1: Write `index.html`** (a deterministic fake widget: 3x3 tile grid + a hidden token field set on correct selection)

```html
<!doctype html>
<html>
<head><meta charset="utf-8"><title>fake captcha</title></head>
<body>
  <div id="status">unsolved</div>
  <input id="captcha-token" type="hidden" value="">
  <div id="grid" data-instruction="select all buses"
       style="display:grid;grid-template-columns:repeat(3,100px);width:300px">
    <!-- 9 tiles; tiles 0,1,2 are the 'buses' -->
    <div class="tile" data-idx="0" style="width:100px;height:100px;background:#3a7"></div>
    <div class="tile" data-idx="1" style="width:100px;height:100px;background:#3a7"></div>
    <div class="tile" data-idx="2" style="width:100px;height:100px;background:#3a7"></div>
    <div class="tile" data-idx="3" style="width:100px;height:100px;background:#ccc"></div>
    <div class="tile" data-idx="4" style="width:100px;height:100px;background:#ccc"></div>
    <div class="tile" data-idx="5" style="width:100px;height:100px;background:#ccc"></div>
    <div class="tile" data-idx="6" style="width:100px;height:100px;background:#ccc"></div>
    <div class="tile" data-idx="7" style="width:100px;height:100px;background:#ccc"></div>
    <div class="tile" data-idx="8" style="width:100px;height:100px;background:#ccc"></div>
  </div>
  <button id="verify">Verify</button>
  <script>
    const selected = new Set();
    document.querySelectorAll('.tile').forEach(t => t.addEventListener('click', () => {
      const i = t.dataset.idx; selected.has(i) ? selected.delete(i) : selected.add(i);
      t.style.outline = selected.has(i) ? '3px solid red' : 'none';
    }));
    document.getElementById('verify').addEventListener('click', () => {
      const want = new Set(['0','1','2']);
      const ok = selected.size === want.size && [...want].every(x => selected.has(x));
      if (ok) {
        document.getElementById('captcha-token').value = 'FAKE_TOKEN_OK';
        document.getElementById('status').textContent = 'solved';
      }
    });
    // expose webdriver flag so stealth e2e can assert it is patched
    window.__wd = navigator.webdriver;
  </script>
</body>
</html>
```

- [ ] **Step 2: Commit**

```bash
git add test/e2e/testpage
git commit -m "test(e2e): local fake-captcha page with hidden token field"
```

### Task 21: Fake LLM HTTP server for e2e

**Files:**
- Create: `test/e2e/fakellm_test.go` (build tag `e2e`) — an OpenAI-compatible `httptest`-style server returning a fixed grid selection `[0,1,2]`.

- [ ] **Step 1: Write the fake LLM**

```go
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
```

- [ ] **Step 2: Commit**

```bash
git add test/e2e/fakellm_test.go
git commit -m "test(e2e): openai-compatible fake LLM returning fixed tiles"
```

### Task 22: MCP stdio client + server-launch e2e (no browser)

**Files:**
- Create: `test/e2e/server_test.go` (build tag `e2e`)

This launches the built binary configured with `provider=openai-compatible` + `base_url=<fake LLM>`, connects an MCP client over the child's stdio, calls `solve_grid_captcha`, and asserts tiles `[0,1,2]`.

- [ ] **Step 1: Write the test** (uses the SDK's client over a command transport)

```go
//go:build e2e

package e2e

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSolveGridOverStdio(t *testing.T) {
	llmSrv := startFakeLLM()
	defer llmSrv.Close()

	// Build once (Makefile target or go build in TestMain); assume bin exists.
	cmd := exec.Command("../../bin/captcha-solver-mcp")
	cmd.Env = append(cmd.Environ(),
		"CAPTCHA_LLM_PROVIDER=openai-compatible",
		"CAPTCHA_LLM_BASE_URL="+llmSrv.URL,
		"CAPTCHA_LLM_API_KEY=test",
		"CAPTCHA_LLM_MODEL=fake",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "solve_grid_captcha",
		Arguments: map[string]any{
			"screenshot":   "iVBORw0KGgo=", // PNG magic; vision is faked
			"instruction":  "select all buses",
			"rows":         3,
			"cols":         3,
			"image_width":  300,
			"image_height": 300,
		},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	// StructuredContent carries the typed Out; assert tiles via the JSON map.
	assertTiles(t, res, []int{0, 1, 2})
}
```

> Note: the exact client API surface (`mcp.NewClient`, `client.Connect`, `mcp.CommandTransport`, reading `res.StructuredContent`) must be confirmed against the go-sdk v1.2.0 client docs at implementation (query-docs: "go-sdk client Connect CommandTransport CallTool read structured result"). `assertTiles` is a small helper that unmarshals `res.StructuredContent` and compares. Implement `assertTiles` in the same file.

- [ ] **Step 2: Add `TestMain` to build the binary**

```go
//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	build := exec.Command("go", "build", "-o", "../../bin/captcha-solver-mcp", "../../cmd/captcha-solver-mcp")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}
```

- [ ] **Step 3: Run + commit**

Run: `make e2e`
Expected: `TestSolveGridOverStdio` PASS — server launched, fake LLM served, tiles `[0,1,2]` returned through the real MCP transport.
```bash
git add test/e2e/server_test.go test/e2e/main_test.go
git commit -m "test(e2e): solve_grid over real MCP stdio with fake LLM"
```

### Task 23: Browser-loop e2e — Playwright (Chromium)

**Files:**
- Create: `test/e2e/playwright/run.mjs` (Node script: serve testpage, inject stealth script from MCP, screenshot grid, call MCP, click tiles via humanized plan, assert token)
- Create: `test/e2e/playwright_test.go` (build tag `e2e`; shells out to the Node script, asserts exit 0)
- Update: `package.json` (devDeps: `playwright`)

- [ ] **Step 1: Write `run.mjs`**

The script: (1) starts a static file server for `testpage/`, (2) launches the MCP server as a child and speaks MCP over stdio via a tiny JSON-RPC client (or via `@modelcontextprotocol/sdk` if added), (3) `get_stealth_script` → `page.addInitScript`, (4) navigate, screenshot `#grid`, (5) `solve_grid_captcha` with width/height 300, (6) `plan_interaction` with the returned centroids, (7) replay clicks, click Verify, (8) assert `#captcha-token` == `FAKE_TOKEN_OK` and `window.__wd === undefined`.

```javascript
import { chromium } from 'playwright';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const pageDir = path.join(__dirname, '..', 'testpage');

// 1. static server
const server = http.createServer((req, res) => {
  const f = path.join(pageDir, req.url === '/' ? 'index.html' : req.url);
  fs.readFile(f, (err, data) => {
    if (err) { res.writeHead(404); res.end(); return; }
    res.end(data);
  });
}).listen(0);
const port = server.address().port;

// 2. minimal MCP stdio client (newline-delimited JSON-RPC)
const mcp = spawn(path.join(__dirname, '..', '..', '..', 'bin', 'captcha-solver-mcp'), {
  env: { ...process.env,
    CAPTCHA_LLM_PROVIDER: 'openai-compatible',
    CAPTCHA_LLM_BASE_URL: process.env.FAKE_LLM_URL,
    CAPTCHA_LLM_API_KEY: 'test', CAPTCHA_LLM_MODEL: 'fake' },
  stdio: ['pipe', 'pipe', 'inherit'],
});
let idc = 0; const pending = new Map(); let buf = '';
mcp.stdout.on('data', (d) => {
  buf += d.toString();
  let nl;
  while ((nl = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, nl); buf = buf.slice(nl + 1);
    if (!line.trim()) continue;
    const msg = JSON.parse(line);
    if (msg.id !== undefined && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); }
  }
});
function rpc(method, params) {
  const id = ++idc;
  return new Promise((resolve) => {
    pending.set(id, resolve);
    mcp.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  });
}
async function callTool(name, args) {
  const r = await rpc('tools/call', { name, arguments: args });
  return r.result.structuredContent;
}

await rpc('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'e2e', version: '0' } });
mcp.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');

// 3. stealth script
const stealth = await callTool('get_stealth_script', { engine: 'playwright' });

const browser = await chromium.launch();
const page = await browser.newPage();
await page.addInitScript(stealth.script);
await page.goto(`http://localhost:${port}/`);

const grid = page.locator('#grid');
const shot = (await grid.screenshot()).toString('base64');
const instruction = await grid.getAttribute('data-instruction');

const sol = await callTool('solve_grid_captcha', {
  screenshot: shot, instruction, rows: 3, cols: 3, image_width: 300, image_height: 300,
});

const plan = await callTool('plan_interaction', {
  actions: sol.centroids.map((c) => ({ type: 'click', x: c.x, y: c.y })),
});

const box = await grid.boundingBox();
// replay: for each 'down' step, click the tile at that grid-relative point
for (const step of plan.steps) {
  if (step.type === 'down') {
    await page.mouse.click(box.x + step.x, box.y + step.y);
  }
}
await page.click('#verify');

const token = await page.inputValue('#captcha-token');
const wd = await page.evaluate(() => window.__wd);
await browser.close(); mcp.kill(); server.close();

if (token !== 'FAKE_TOKEN_OK') { console.error('token mismatch:', token); process.exit(1); }
if (wd !== undefined) { console.error('webdriver not patched:', wd); process.exit(1); }
console.log('PASS');
```

- [ ] **Step 2: Write `playwright_test.go`**

```go
//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestPlaywrightGridLoop(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	llm := startFakeLLM()
	defer llm.Close()

	cmd := exec.Command("node", "playwright/run.mjs")
	cmd.Env = append(cmd.Environ(), "FAKE_LLM_URL="+llm.URL)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("playwright loop failed: %v", err)
	}
}
```

- [ ] **Step 3: Install Playwright + browsers**

Run:
```bash
npm init -y
npm install -D playwright
npx playwright install chromium
```

- [ ] **Step 4: Run + commit**

Run: `make e2e`
Expected: `TestPlaywrightGridLoop` PASS — full screenshot→solve→humanized-click→token loop offline; stealth script verified (`navigator.webdriver === undefined`).
```bash
git add test/e2e/playwright test/e2e/playwright_test.go package.json package-lock.json
git commit -m "test(e2e): playwright grid-loop with stealth + humanized clicks"
```

### Task 24: Browser-loop e2e — Lightpanda (detect/DOM)

**Files:**
- Create: `test/e2e/lightpanda/run.mjs` (drives Lightpanda via CDP: load page, read DOM HTML, call `detect_captcha`, assert detection)
- Create: `test/e2e/lightpanda_test.go` (build tag `e2e`; skips if `lightpanda` binary absent)

- [ ] **Step 1: Write `run.mjs`** (Lightpanda is CDP-compatible; use Playwright's `chromium.connectOverCDP` or puppeteer-core to attach; this path tests detection from DOM, not canvas screenshots)

```javascript
import { chromium } from 'playwright';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const pageDir = path.join(__dirname, '..', 'testpage');
// Serve a recaptcha-marker page for detection
const html = `<div class="g-recaptcha" data-sitekey="6Lc_e2e"></div>`;
const server = http.createServer((_, res) => res.end(html)).listen(0);
const port = server.address().port;

// Launch Lightpanda in CDP server mode (adjust flag per installed version).
const lp = spawn('lightpanda', ['serve', '--host', '127.0.0.1', '--port', '9222'], { stdio: 'inherit' });
await new Promise((r) => setTimeout(r, 800));

// MCP client (same minimal RPC as playwright/run.mjs — factor into a shared module at impl time)
// ... initialize MCP, then:
const browser = await chromium.connectOverCDP('http://127.0.0.1:9222');
const page = await browser.newPage();
await page.goto(`http://localhost:${port}/`);
const dom = await page.content();
const det = await callTool('detect_captcha', { html: dom });
await browser.close(); lp.kill(); server.close();

if (!det.detected.some((d) => d.type === 'recaptcha_v2')) { console.error('no detection', det); process.exit(1); }
console.log('PASS');
```

> Implementation note: the MCP client boilerplate is identical to Task 23 — extract `test/e2e/mcpclient.mjs` and import it in both `run.mjs` scripts (DRY). Confirm the exact Lightpanda CDP launch flags against the installed binary's `--help` at implementation.

- [ ] **Step 2: Write `lightpanda_test.go`**

```go
//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestLightpandaDetect(t *testing.T) {
	if _, err := exec.LookPath("lightpanda"); err != nil {
		t.Skip("lightpanda not installed")
	}
	llm := startFakeLLM()
	defer llm.Close()
	cmd := exec.Command("node", "lightpanda/run.mjs")
	cmd.Env = append(cmd.Environ(), "FAKE_LLM_URL="+llm.URL)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("lightpanda detect failed: %v", err)
	}
}
```

- [ ] **Step 3: Run + commit**

Run: `make e2e` (Lightpanda test skips cleanly if the binary is absent).
```bash
git add test/e2e/lightpanda test/e2e/lightpanda_test.go test/e2e/mcpclient.mjs
git commit -m "test(e2e): lightpanda DOM detection path"
```

---

## Phase 11 — Gated live e2e (opt-in)

### Task 25: Live tier against real demo pages

**Files:**
- Create: `test/e2e/live_test.go` (build tags `e2e,live`)

- [ ] **Step 1: Write the live test** (skips unless `CAPTCHA_E2E_LIVE=1` and real creds present)

```go
//go:build e2e && live

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestLiveRecaptchaDemo(t *testing.T) {
	if os.Getenv("CAPTCHA_E2E_LIVE") != "1" {
		t.Skip("set CAPTCHA_E2E_LIVE=1 to run live tests")
	}
	if os.Getenv("CAPTCHA_LLM_API_KEY") == "" || os.Getenv("CAPTCHA_LLM_MODEL") == "" {
		t.Skip("live test needs real CAPTCHA_LLM_* creds")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	// Drives the real Google reCAPTCHA v2 demo page with a real LLM + Chromium.
	// Best-effort: token captchas are stochastic; treat a single failure as
	// non-fatal unless CAPTCHA_E2E_LIVE_STRICT=1.
	cmd := exec.Command("node", "playwright/run-live.mjs")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil && os.Getenv("CAPTCHA_E2E_LIVE_STRICT") == "1" {
		t.Fatalf("live recaptcha demo failed: %v", err)
	}
	if err != nil {
		t.Logf("live demo did not solve this run (expected occasionally): %v", err)
	}
}
```

- [ ] **Step 2: Write `playwright/run-live.mjs`**

Adapt `run.mjs` to: navigate to the public reCAPTCHA v2 demo, click the checkbox, when the image challenge appears screenshot the challenge `iframe`, loop `solve_grid_captcha` (handling `recheck_recommended` by re-screenshotting), use `generate_fingerprint` + `get_stealth_script` + `plan_interaction`, and read `textarea[name="g-recaptcha-response"]`. Real LLM via the server's configured provider (do NOT point at the fake LLM). Exit 0 if a token appears.

- [ ] **Step 3: Document running it**

Run (manual): `CAPTCHA_E2E_LIVE=1 CAPTCHA_LLM_PROVIDER=anthropic CAPTCHA_LLM_API_KEY=... CAPTCHA_LLM_MODEL=claude-opus-4-8 make e2e-live`
Expected: skipped by default; when enabled, attempts a real solve (best-effort).

- [ ] **Step 4: Commit**

```bash
git add test/e2e/live_test.go test/e2e/playwright/run-live.mjs
git commit -m "test(e2e): gated live tier against real recaptcha demo"
```

---

## Phase 12 — Examples + docs

### Task 26: Runnable examples + README

**Files:**
- Create: `examples/playwright/solve-recaptcha.mjs` (annotated end-to-end recipe)
- Create: `examples/lightpanda/detect.mjs`
- Rewrite: `README.md`

- [ ] **Step 1: Write `examples/playwright/solve-recaptcha.mjs`**

A documented version of the Task-23 loop, parameterized by env, with comments explaining the brain/hands split, the dynamic-grid re-screenshot loop, and where the token is read. No assertions — it's a copy-paste starting point.

- [ ] **Step 2: Write `examples/lightpanda/detect.mjs`**

A short script: connect to Lightpanda over CDP, load a URL, call `detect_captcha`, print the result.

- [ ] **Step 3: Rewrite `README.md`**

Cover: what it is / isn't (brain-only, no token minting, v3/Turnstile detect-only); **authorized-use statement**; install/build; env vars table; each tool's input/output; the Playwright/Lightpanda recipe; testing (`make test` / `make e2e` / `make e2e-live`); the TLS/JA3 advisory limitation.

- [ ] **Step 4: Final full verification**

Run:
```bash
make tidy && make lint && go test ./... && make build && make e2e
```
Expected: tidy clean, lint clean, unit PASS, build OK, deterministic e2e PASS (Lightpanda test skips if absent).

- [ ] **Step 5: Commit**

```bash
git add examples README.md
git commit -m "docs: runnable examples and README with authorized-use scope"
```

---

## Acceptance criteria (maps to spec §)

- §3/§1 brain-only/solver-only: server imports no browser driver; all tools take inputs and return data (Tasks 16–17). ✓
- §5 env-driven providers incl. fail-fast: Task 1 + Task 5. ✓
- §6 nine tools (detect, text, audio, grid, rotation, solver_info, get_stealth_script, generate_fingerprint, plan_interaction): Tasks 7–16. ✓
- §6 input formats base64/path/url/data-uri: Task 6. ✓
- §6.4 grid returns tiles + centroids + recheck: Task 10. ✓
- §7 v3/Turnstile detect-only (`llm_solvable:false`): Task 7. ✓
- §8 four stealth capabilities: Tasks 12–15. ✓
- §6.3 audio "unsupported" honest error on anthropic: Tasks 4, 16. ✓
- §10 strict-retry, no fabricated answers, confidence returned: Tasks 8, 10. ✓
- §11 unit (fake LLM) + deterministic e2e (both engines, local page) + gated live: Tasks 2–24 (unit), 20–24 (deterministic e2e), 25 (live). ✓
- §12 golangci clean incl. fieldalignment, gosec on tests, no GitHub Actions: Tasks 0, 18; Makefile local-only. ✓
- §9 Playwright + Lightpanda recipes shipped: Task 26. ✓

## Notes for the implementer
- **Verify two SDK surfaces against go-sdk v1.2.0 docs before coding the tasks that use them:** (a) the **client** API in Task 22 (`mcp.NewClient` / `Connect` / `CommandTransport` / reading `StructuredContent`) and (b) any `mcp.AddTool` handler-signature nuance in Task 16. The server/`AddTool`/`StdioTransport` shapes are already grounded.
- **Gemini (Task 19) is the one provider with an unverified wire format** — its grounding gate is mandatory; do not ship without the context7 check.
- **Evasion JS (Task 13)** is a minimal in-repo set; before release, reconcile with a current open-source evasion bundle and record source + license in `internal/stealth/assets/NOTICE`.
- Keep commit bodies free of "breaking"/"feat" wording beyond the conventional-commit type prefix (semver-generator fuzzy-matches bodies).
