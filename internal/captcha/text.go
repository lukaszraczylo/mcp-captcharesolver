package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/imageutil"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// TextOptions tunes text-image OCR.
type TextOptions struct {
	Hint          string
	Charset       string
	Length        int
	Samples       int
	Upscale       int
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
	p += "Carefully distinguish visually similar glyphs: 0 vs O, 1 vs l vs I, 5 vs S, 2 vs Z, 8 vs B. "
	p += `Respond ONLY with compact JSON: {"text":"<chars>","confidence":<0..1>}.`
	if strict {
		p += " You MUST output valid JSON and nothing else. Do not explain."
	}
	return p
}

// SolveText runs OCR via the vision model with one stricter retry on parse
// failure. When o.Samples > 1 it self-consistency votes across N solves,
// returning the most frequent text (tie-break by highest confidence).
func SolveText(ctx context.Context, c llm.Client, img []byte, mediaType string, o TextOptions) (TextResult, error) {
	if o.Upscale > 1 {
		if up, mt, err := imageutil.Upscale(img, o.Upscale); err == nil {
			img, mediaType = up, mt
		}
	}
	images := []llm.Image{{MediaType: mediaType, Data: img}}

	if o.Samples <= 1 {
		return solveTextOnce(ctx, c, images, o, llm.Options{})
	}

	opts := llm.Options{Temperature: 0.5}
	results := make([]TextResult, 0, o.Samples)
	for i := 0; i < o.Samples; i++ {
		res, err := solveTextOnce(ctx, c, images, o, opts)
		if err != nil {
			continue
		}
		results = append(results, res)
	}
	if len(results) == 0 {
		return TextResult{}, fmt.Errorf("model did not return parseable captcha text")
	}
	return voteText(results), nil
}

// solveTextOnce performs a single solve with one stricter retry on parse fail.
func solveTextOnce(ctx context.Context, c llm.Client, images []llm.Image, o TextOptions, opts llm.Options) (TextResult, error) {
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := c.Vision(ctx, textPrompt(o, attempt > 0), images, opts)
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

// voteText returns the most frequent text across results. Ties break by the
// highest confidence seen for the candidate; remaining ties break
// deterministically by text. Confidence is the fraction of results that agree
// with the winner.
func voteText(results []TextResult) TextResult {
	counts := make(map[string]int)
	bestConf := make(map[string]float64)
	for _, r := range results {
		counts[r.Text]++
		if r.Confidence > bestConf[r.Text] {
			bestConf[r.Text] = r.Confidence
		}
	}
	texts := make([]string, 0, len(counts))
	for t := range counts {
		texts = append(texts, t)
	}
	sort.Slice(texts, func(i, j int) bool {
		a, b := texts[i], texts[j]
		if counts[a] != counts[b] {
			return counts[a] > counts[b]
		}
		if bestConf[a] != bestConf[b] {
			return bestConf[a] > bestConf[b]
		}
		return a < b
	})
	winner := texts[0]
	return TextResult{
		Text:       winner,
		Confidence: float64(counts[winner]) / float64(len(results)),
	}
}
