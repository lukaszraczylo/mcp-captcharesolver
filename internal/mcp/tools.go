// Package mcpserver wires captcha + stealth functions to MCP tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lukaszraczylo/captcha-solver-mcp/internal/captcha"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/imageutil"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/llm"
	"github.com/lukaszraczylo/captcha-solver-mcp/internal/stealth"
)

// Handlers holds dependencies shared by all tool handlers.
type Handlers struct {
	llm      llm.Client
	provider string
	model    string
}

// New constructs Handlers.
func New(c llm.Client, provider, model string) *Handlers {
	return &Handlers{llm: c, provider: provider, model: model}
}

// ---- detect_captcha ----

type DetectIn struct {
	HTML       string `json:"html,omitempty" jsonschema:"page HTML to scan for captcha markers"`
	URL        string `json:"url,omitempty" jsonschema:"page URL (informational)"`
	Screenshot string `json:"screenshot,omitempty" jsonschema:"base64/path/url screenshot, used only if HTML is inconclusive"`
}
type DetectOut struct {
	Detected []captcha.Detection `json:"detected"`
	Summary  string              `json:"summary"`
}

func (h *Handlers) detect(_ context.Context, _ *mcp.CallToolRequest, in DetectIn) (*mcp.CallToolResult, DetectOut, error) {
	det := captcha.DetectHTML(in.HTML)
	summary := fmt.Sprintf("%d captcha(s) detected", len(det))
	return nil, DetectOut{Detected: det, Summary: summary}, nil
}

// ---- solve_text_captcha ----

type SolveTextIn struct {
	Image         string `json:"image" jsonschema:"base64/path/url/data-uri of the text captcha image"`
	Hint          string `json:"hint,omitempty"`
	Charset       string `json:"charset,omitempty"`
	Length        int    `json:"length,omitempty"`
	Samples       int    `json:"samples,omitempty" jsonschema:"run N times and majority-vote the text (default 1); costs N× calls"`
	Upscale       int    `json:"upscale,omitempty" jsonschema:"upscale factor 1-6 before OCR (helps small/blurry text)"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
}

func (h *Handlers) solveText(ctx context.Context, _ *mcp.CallToolRequest, in SolveTextIn) (*mcp.CallToolResult, captcha.TextResult, error) {
	data, mt, err := imageutil.Decode(in.Image)
	if err != nil {
		return nil, captcha.TextResult{}, err
	}
	res, err := captcha.SolveText(ctx, h.llm, data, mt, captcha.TextOptions{
		Hint: in.Hint, Charset: in.Charset, Length: in.Length, CaseSensitive: in.CaseSensitive,
		Samples: in.Samples, Upscale: in.Upscale,
	})
	return nil, res, err
}

// ---- solve_audio_captcha ----

type SolveAudioIn struct {
	Audio    string `json:"audio" jsonschema:"base64/path/url of the audio captcha"`
	Language string `json:"language,omitempty"`
}

func (h *Handlers) solveAudio(ctx context.Context, _ *mcp.CallToolRequest, in SolveAudioIn) (*mcp.CallToolResult, captcha.AudioResult, error) {
	data, mt, err := imageutil.Decode(in.Audio)
	if err != nil {
		return nil, captcha.AudioResult{}, err
	}
	filename := "audio.wav"
	if mt == "audio/mpeg" {
		filename = "audio.mp3"
	}
	res, err := captcha.SolveAudio(ctx, h.llm, data, filename, in.Language)
	var unsupported llm.ErrUnsupported
	if errors.As(err, &unsupported) {
		return nil, captcha.AudioResult{}, fmt.Errorf("audio captcha unsupported: configured provider %q has no transcription; set CAPTCHA_LLM_PROVIDER=openai|gemini and CAPTCHA_LLM_AUDIO_MODEL", h.provider)
	}
	return nil, res, err
}

// ---- solve_grid_captcha ----

type SolveGridIn struct {
	Screenshot  string `json:"screenshot" jsonschema:"base64/path/url of the challenge grid image"`
	Instruction string `json:"instruction" jsonschema:"the challenge text, e.g. 'select all buses'"`
	CaptchaType string `json:"captcha_type,omitempty" jsonschema:"recaptcha | hcaptcha"`
	Rows        int    `json:"rows" jsonschema:"grid rows (e.g. 3)"`
	Cols        int    `json:"cols" jsonschema:"grid cols (e.g. 3)"`
	ImageWidth  int    `json:"image_width,omitempty" jsonschema:"screenshot width in px, enables centroid output"`
	ImageHeight int    `json:"image_height,omitempty" jsonschema:"screenshot height in px, enables centroid output"`
	Samples     int    `json:"samples,omitempty" jsonschema:"run N times and majority-vote tiles (default 1; 3 ≈ Success@3); costs N× calls"`
	Upscale     int    `json:"upscale,omitempty" jsonschema:"upscale factor 1-6 before solving (helps small tiles)"`
	Annotate    bool   `json:"annotate,omitempty" jsonschema:"overlay numbered grid cells to disambiguate"`
}

func (h *Handlers) solveGrid(ctx context.Context, _ *mcp.CallToolRequest, in SolveGridIn) (*mcp.CallToolResult, captcha.GridResult, error) {
	data, mt, err := imageutil.Decode(in.Screenshot)
	if err != nil {
		return nil, captcha.GridResult{}, err
	}
	res, err := captcha.SolveGrid(ctx, h.llm, data, mt, captcha.GridOptions{
		Instruction: in.Instruction, CaptchaType: in.CaptchaType,
		Rows: in.Rows, Cols: in.Cols, ImageWidth: in.ImageWidth, ImageHeight: in.ImageHeight,
		Samples: in.Samples, Upscale: in.Upscale, Annotate: in.Annotate,
	})
	return nil, res, err
}

// ---- solve_rotation_captcha ----

type SolveRotationIn struct {
	Screenshot  string `json:"screenshot" jsonschema:"base64/path/url of the rotation/choice puzzle"`
	Instruction string `json:"instruction,omitempty"`
}

func (h *Handlers) solveRotation(ctx context.Context, _ *mcp.CallToolRequest, in SolveRotationIn) (*mcp.CallToolResult, captcha.RotationResult, error) {
	data, mt, err := imageutil.Decode(in.Screenshot)
	if err != nil {
		return nil, captcha.RotationResult{}, err
	}
	res, err := captcha.SolveRotation(ctx, h.llm, data, mt, in.Instruction)
	return nil, res, err
}

// ---- stealth tools ----

type StealthScriptIn struct {
	Seed    uint64 `json:"seed,omitempty" jsonschema:"use the same seed as generate_fingerprint to keep the injected script and applied profile consistent"`
	OS      string `json:"os,omitempty" jsonschema:"windows | macos | linux — use the same os/browser/locale/seed as generate_fingerprint, or read the returned fingerprint, to keep the injected script and applied profile consistent"`
	Browser string `json:"browser,omitempty" jsonschema:"chrome"`
	Locale  string `json:"locale,omitempty" jsonschema:"BCP-47 locale e.g. en-US, en-GB — use the same locale as generate_fingerprint for a coherent profile"`
	Engine  string `json:"engine,omitempty" jsonschema:"playwright | cdp | generic"`
}

func (h *Handlers) stealthScript(_ context.Context, _ *mcp.CallToolRequest, in StealthScriptIn) (*mcp.CallToolResult, stealth.ScriptResult, error) {
	return nil, stealth.StealthScript(stealth.ScriptParams{
		OS:      in.OS,
		Browser: in.Browser,
		Locale:  in.Locale,
		Seed:    in.Seed,
		Engine:  in.Engine,
	}), nil
}

type FingerprintIn struct {
	OS      string `json:"os,omitempty" jsonschema:"windows | macos | linux"`
	Browser string `json:"browser,omitempty" jsonschema:"chrome (firefox not yet differentiated)"`
	Locale  string `json:"locale,omitempty"`
	Seed    uint64 `json:"seed,omitempty" jsonschema:"seed for a stable identity across runs"`
}
type FingerprintOut struct {
	Fingerprint stealth.Fingerprint `json:"fingerprint"`
	HeaderSet   stealth.HeaderSet   `json:"header_set"`
}

func (h *Handlers) fingerprint(_ context.Context, _ *mcp.CallToolRequest, in FingerprintIn) (*mcp.CallToolResult, FingerprintOut, error) {
	fp := stealth.GenerateFingerprint(stealth.FingerprintParams{OS: in.OS, Browser: in.Browser, Locale: in.Locale, Seed: in.Seed})
	return nil, FingerprintOut{Fingerprint: fp, HeaderSet: stealth.HeadersFor(fp)}, nil
}

type PlanInteractionIn struct {
	Actions []stealth.Action `json:"actions" jsonschema:"actions to humanize (click | type)"`
	Seed    uint64           `json:"seed,omitempty"`
}

func (h *Handlers) planInteraction(_ context.Context, _ *mcp.CallToolRequest, in PlanInteractionIn) (*mcp.CallToolResult, stealth.InteractionPlan, error) {
	return nil, stealth.PlanInteraction(stealth.InteractionParams{Actions: in.Actions, Seed: in.Seed}), nil
}

// ---- solver_info ----

type SolverInfoOut struct {
	Provider     string            `json:"provider"`
	Model        string            `json:"model"`
	Capabilities map[string]bool   `json:"capabilities"`
	Notes        map[string]string `json:"notes"`
}

func (h *Handlers) solverInfo(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SolverInfoOut, error) {
	audio := h.provider == "openai" || h.provider == "openai-compatible" || h.provider == "gemini"
	return nil, SolverInfoOut{
		Provider: h.provider, Model: h.model,
		Capabilities: map[string]bool{
			"text": true, "grid": true, "rotation": true, "audio": audio, "detect": true,
			"stealth": true,
		},
		Notes: map[string]string{
			"recaptcha_v3": "detect-only, not LLM-solvable",
			"turnstile":    "detect-only, not LLM-solvable",
		},
	}, nil
}
