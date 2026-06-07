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
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
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
