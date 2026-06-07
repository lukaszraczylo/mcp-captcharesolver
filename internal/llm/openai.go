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
	if err := mw.WriteField("model", model); err != nil {
		return "", err
	}
	if opts.Language != "" {
		if err := mw.WriteField("language", opts.Language); err != nil {
			return "", err
		}
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
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
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return json.Unmarshal(raw, out)
}
