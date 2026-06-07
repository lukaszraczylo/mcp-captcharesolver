package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

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
	Samples     int
	Upscale     int
	Annotate    bool
}

// GridResult lists tiles to click (row-major 0-based) plus optional pixel
// centroids when grid dimensions are known.
type GridResult struct {
	Tiles              []int             `json:"tiles"`
	Centroids          []imageutil.Point `json:"centroids,omitempty"`
	Confidence         float64           `json:"confidence"`
	NoTargets          bool              `json:"no_targets"`
	RecheckRecommended bool              `json:"recheck_recommended"`
}

func gridPrompt(o GridOptions, strict bool) string {
	p := fmt.Sprintf(
		"You are solving an image-grid captcha. The image is a %dx%d grid of tiles, "+
			"numbered row-major starting at 0 (top-left) through %d (numbers may be overlaid on the image). "+
			"Instruction: %q. "+
			"Select EVERY cell that contains ANY part of the target, including small or partial portions. "+
			"If no cell contains the target, return an empty tiles array and set no_targets true. "+
			"If this is a dynamic grid where new matching tiles may appear after selection, set recheck_recommended true. ",
		o.Rows, o.Cols, o.Rows*o.Cols-1, o.Instruction)
	p += `Respond ONLY with compact JSON: {"tiles":[..],"confidence":<0..1>,"no_targets":<bool>,"recheck_recommended":<bool>}.`
	if strict {
		p += " You MUST output valid JSON only. No prose."
	}
	return p
}

// SolveGrid asks the vision model which tiles to click and (when grid geometry
// is supplied) maps them to pixel centroids for humanized clicking. When
// o.Samples > 1 it self-consistency votes across N solves, keeping tiles that
// appear in a strict majority of successful samples.
func SolveGrid(ctx context.Context, c llm.Client, img []byte, mediaType string, o GridOptions) (GridResult, error) {
	img, mediaType = preprocessGrid(img, mediaType, o)
	images := []llm.Image{{MediaType: mediaType, Data: img}}

	if o.Samples <= 1 {
		res, err := solveGridOnce(ctx, c, images, o, llm.Options{})
		if err != nil {
			return GridResult{}, err
		}
		res.Centroids = mapCentroids(res.Tiles, o)
		return res, nil
	}

	opts := llm.Options{Temperature: 0.5}
	results := make([]GridResult, 0, o.Samples)
	for i := 0; i < o.Samples; i++ {
		res, err := solveGridOnce(ctx, c, images, o, opts)
		if err != nil {
			continue
		}
		results = append(results, res)
	}
	if len(results) == 0 {
		return GridResult{}, fmt.Errorf("model did not return parseable tile selection")
	}
	voted := voteGrid(results)
	voted.Centroids = mapCentroids(voted.Tiles, o)
	return voted, nil
}

// preprocessGrid optionally upscales then annotates the screenshot. On any
// preprocessing error it falls back to the original bytes so a solve is never
// failed by an optional enhancement.
func preprocessGrid(img []byte, mediaType string, o GridOptions) ([]byte, string) {
	if o.Upscale > 1 {
		if up, mt, err := imageutil.Upscale(img, o.Upscale); err == nil {
			img, mediaType = up, mt
		}
	}
	if o.Annotate {
		if ann, err := imageutil.AnnotateGrid(img, o.Rows, o.Cols); err == nil {
			img, mediaType = ann, "image/png"
		}
	}
	return img, mediaType
}

// solveGridOnce performs a single solve with one stricter retry on parse fail.
func solveGridOnce(ctx context.Context, c llm.Client, images []llm.Image, o GridOptions, opts llm.Options) (GridResult, error) {
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := c.Vision(ctx, gridPrompt(o, attempt > 0), images, opts)
		if err != nil {
			return GridResult{}, err
		}
		var res GridResult
		if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err == nil && res.Tiles != nil {
			return res, nil
		}
	}
	return GridResult{}, fmt.Errorf("model did not return parseable tile selection")
}

// voteGrid keeps tiles appearing in a strict majority of successful samples.
// Confidence is the average agreement of the chosen tiles; NoTargets is true
// when a majority of samples reported no targets.
func voteGrid(results []GridResult) GridResult {
	n := len(results)
	tileCounts := make(map[int]int)
	noTargets := 0
	recheck := 0
	for _, r := range results {
		seen := make(map[int]struct{}, len(r.Tiles))
		for _, t := range r.Tiles {
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			tileCounts[t]++
		}
		if r.NoTargets {
			noTargets++
		}
		if r.RecheckRecommended {
			recheck++
		}
	}

	tiles := make([]int, 0, len(tileCounts))
	var agreeSum float64
	for t, cnt := range tileCounts {
		if cnt*2 > n { // strict majority: more than half
			tiles = append(tiles, t)
			agreeSum += float64(cnt) / float64(n)
		}
	}
	sort.Ints(tiles)

	var conf float64
	if len(tiles) > 0 {
		conf = agreeSum / float64(len(tiles))
	}
	return GridResult{
		Tiles:              tiles,
		Confidence:         conf,
		NoTargets:          noTargets*2 > n,
		RecheckRecommended: recheck*2 > n,
	}
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
