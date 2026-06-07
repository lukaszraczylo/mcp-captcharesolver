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
