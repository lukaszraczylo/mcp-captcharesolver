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
