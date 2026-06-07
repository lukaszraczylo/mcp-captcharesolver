package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Register attaches all tools to the given MCP server.
func (h *Handlers) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "detect_captcha", Description: "Detect captcha type/sitekey from page HTML or screenshot. Flags v3/Turnstile as not LLM-solvable."}, h.detect)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_text_captcha", Description: "OCR a distorted-text image captcha and return the characters."}, h.solveText)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_audio_captcha", Description: "Transcribe an audio captcha (requires an audio-capable provider)."}, h.solveAudio)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_grid_captcha", Description: "reCAPTCHA v2 / hCaptcha: return which grid tiles to click (and centroids). Caller clicks + harvests the token."}, h.solveGrid)
	mcp.AddTool(s, &mcp.Tool{Name: "solve_rotation_captcha", Description: "Best-effort FunCaptcha/rotation puzzle solver."}, h.solveRotation)
	mcp.AddTool(s, &mcp.Tool{Name: "get_stealth_script", Description: "Return an evasion JS init script the caller injects, plus apply instructions."}, h.stealthScript)
	mcp.AddTool(s, &mcp.Tool{Name: "generate_fingerprint", Description: "Generate a coherent browser fingerprint + matching header set (optionally seeded)."}, h.fingerprint)
	mcp.AddTool(s, &mcp.Tool{Name: "plan_interaction", Description: "Convert target clicks/typing into a humanized mouse/keyboard timeline."}, h.planInteraction)
	mcp.AddTool(s, &mcp.Tool{Name: "solver_info", Description: "Report configured provider/model and per-type capabilities."}, h.solverInfo)
}
