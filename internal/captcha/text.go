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
