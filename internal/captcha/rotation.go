package captcha

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
)

// RotationResult is a best-effort FunCaptcha/rotation answer.
type RotationResult struct {
	Choice     int     `json:"choice"`
	Rotations  int     `json:"rotations"`
	Confidence float64 `json:"confidence"`
}

// SolveRotation handles FunCaptcha-style rotation/choice puzzles (best-effort).
func SolveRotation(ctx context.Context, c llm.Client, img []byte, mediaType, instruction string) (RotationResult, error) {
	prompt := fmt.Sprintf(
		"You are solving a rotation/choice puzzle captcha. Instruction: %q. "+
			"If asked to pick one image, return its 0-based index as choice. "+
			"If asked to rotate, return the number of clockwise steps as rotations. "+
			"First reason step-by-step about the correct orientation/choice, then give the final answer "+
			`as the last thing you output: compact JSON {"choice":<int>,"rotations":<int>,"confidence":<0..1>}.`,
		instruction)
	raw, err := c.Vision(ctx, prompt, []llm.Image{{MediaType: mediaType, Data: img}}, llm.Options{})
	if err != nil {
		return RotationResult{}, err
	}
	var res RotationResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return RotationResult{}, fmt.Errorf("model did not return parseable rotation answer")
	}
	return res, nil
}
