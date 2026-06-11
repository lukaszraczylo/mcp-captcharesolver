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
				LogLevel: "info", Transport: "stdio", HTTPAddr: ":8080", HTTPPath: "/mcp",
				MaxTokens: 1024, Timeout: 60 * time.Second,
			},
		},
		{
			name: "valid http transport",
			env: map[string]string{
				"CAPTCHA_LLM_PROVIDER": "openai-compatible",
				"CAPTCHA_LLM_BASE_URL": "http://localhost:11434/v1",
				"CAPTCHA_LLM_MODEL":    "qwen2.5vl",
				"CAPTCHA_TRANSPORT":    "http",
				"CAPTCHA_HTTP_ADDR":    "127.0.0.1:9090",
				"CAPTCHA_HTTP_PATH":    "/mcp",
			},
			want: Config{
				Provider: "openai-compatible", BaseURL: "http://localhost:11434/v1",
				Model:    "qwen2.5vl",
				LogLevel: "info", Transport: "http", HTTPAddr: "127.0.0.1:9090", HTTPPath: "/mcp",
				MaxTokens: 1024, Timeout: 60 * time.Second,
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
		{
			name: "bad transport",
			env: map[string]string{
				"CAPTCHA_LLM_PROVIDER": "openai", "CAPTCHA_LLM_API_KEY": "k", "CAPTCHA_LLM_MODEL": "m",
				"CAPTCHA_TRANSPORT": "quic",
			},
			wantErr: true,
		},
		{
			name: "http transport with relative path",
			env: map[string]string{
				"CAPTCHA_LLM_PROVIDER": "openai", "CAPTCHA_LLM_API_KEY": "k", "CAPTCHA_LLM_MODEL": "m",
				"CAPTCHA_TRANSPORT": "http",
				"CAPTCHA_HTTP_PATH": "mcp",
			},
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
