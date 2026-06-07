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

// generateContent calls POST {base}/v1beta/models/{model}:generateContent.
func (c *geminiClient) generateContent(ctx context.Context, model string, parts []map[string]any) (string, error) {
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": parts},
		},
	}
	if c.maxTokens > 0 {
		body["generationConfig"] = map[string]any{"maxOutputTokens": c.maxTokens}
	}

	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", c.base, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.key)

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
		return "", fmt.Errorf("gemini: empty response")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}

func (c *geminiClient) Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error) {
	model := opts.Model
	if model == "" {
		model = c.model
	}

	parts := []map[string]any{
		{"text": prompt},
	}
	for _, img := range images {
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": img.MediaType,
				"data":      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}

	return c.generateContent(ctx, model, parts)
}

func (c *geminiClient) Transcribe(ctx context.Context, audio []byte, _ string, opts Options) (string, error) {
	model := opts.Model
	if model == "" {
		model = c.audioModel
	}
	if model == "" {
		model = c.model
	}

	parts := []map[string]any{
		{"text": "Transcribe the spoken characters in this audio captcha. Output only the transcription, nothing else."},
		{
			"inline_data": map[string]any{
				"mime_type": "audio/wav",
				"data":      base64.StdEncoding.EncodeToString(audio),
			},
		},
	}

	return c.generateContent(ctx, model, parts)
}
