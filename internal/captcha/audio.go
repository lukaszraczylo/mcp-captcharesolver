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
