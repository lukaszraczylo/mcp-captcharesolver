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
