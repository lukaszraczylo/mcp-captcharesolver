package captcha

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/imageutil"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// GridOptions describes a reCAPTCHA v2 / hCaptcha tile grid challenge.
type GridOptions struct {
	Instruction string
	CaptchaType string // recaptcha | hcaptcha (informational)
	Rows        int
	Cols        int
	ImageWidth  int
	ImageHeight int
}

// GridResult lists tiles to click (row-major 0-based) plus optional pixel
// centroids when grid dimensions are known.
type GridResult struct {
	Tiles              []int             `json:"tiles"`
	Centroids          []imageutil.Point `json:"centroids,omitempty"`
	Confidence         float64           `json:"confidence"`
	RecheckRecommended bool              `json:"recheck_recommended"`
}

func gridPrompt(o GridOptions, strict bool) string {
	p := fmt.Sprintf(
		"You are solving an image-grid captcha. The image is a %dx%d grid of tiles, "+
			"numbered row-major starting at 0 (top-left). Instruction: %q. "+
			"Return the 0-based indices of every tile that matches the instruction. "+
			"If after selection more matching tiles would likely appear (dynamic grid), set recheck_recommended true. "+
			`Respond ONLY with compact JSON: {"tiles":[..],"confidence":<0..1>,"recheck_recommended":<bool>}.`,
		o.Rows, o.Cols, o.Instruction)
	if strict {
		p += " You MUST output valid JSON only. No prose."
	}
	return p
}

// SolveGrid asks the vision model which tiles to click and (when grid geometry
// is supplied) maps them to pixel centroids for humanized clicking.
func SolveGrid(ctx context.Context, c llm.Client, img []byte, mediaType string, o GridOptions) (GridResult, error) {
	images := []llm.Image{{MediaType: mediaType, Data: img}}
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := c.Vision(ctx, gridPrompt(o, attempt > 0), images, llm.Options{})
		if err != nil {
			return GridResult{}, err
		}
		var res GridResult
		if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err == nil && res.Tiles != nil {
			res.Centroids = mapCentroids(res.Tiles, o)
			return res, nil
		}
	}
	return GridResult{}, fmt.Errorf("model did not return parseable tile selection")
}

func mapCentroids(tiles []int, o GridOptions) []imageutil.Point {
	if o.Rows <= 0 || o.Cols <= 0 || o.ImageWidth <= 0 || o.ImageHeight <= 0 {
		return nil
	}
	all := imageutil.GridCentroids(o.ImageWidth, o.ImageHeight, o.Rows, o.Cols)
	out := make([]imageutil.Point, 0, len(tiles))
	for _, idx := range tiles {
		if idx >= 0 && idx < len(all) {
			out = append(out, all[idx])
		}
	}
	return out
}
