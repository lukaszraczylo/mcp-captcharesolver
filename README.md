# captcha-solver-mcp

A browser-agnostic **vision/LLM "brain" + stealth-artifact generator**, exposed as a
[Model Context Protocol](https://modelcontextprotocol.io) (MCP) server over stdio. It looks
at captcha challenges (screenshots, audio, page HTML) and tells the caller *what to do* —
which grid tiles match, what the distorted text says, what the audio says, which rotation
to pick — and it generates the stealth artifacts (evasion init scripts, coherent
fingerprints + headers, humanized interaction timelines) that make automated interaction
look human. It is the **brain**; your automation (Playwright, Lightpanda, or any CDP-driven
browser) is the **hands** that actually click tiles and harvest tokens.

## Authorized use only

This is a **dual-use tool**. It is intended for:

- Testing the anti-bot / captcha integrations on **sites and systems you own or are
  explicitly authorized to test**.
- Accessibility automation (e.g. solving an audio captcha for an assistive workflow).
- Sanctioned, contractually-permitted scraping and QA.

It is **not** for evading protections on sites you do not own or control, for fraud, for
account abuse, or for circumventing access controls. You are responsible for ensuring your
use complies with the target site's terms of service and applicable law. Misuse is on you.

## What it is — and isn't

**It is:** a stateless solver/advisor. Given an input it returns an answer or an artifact.

**It is not** an automation driver:

- It **never drives a browser** — no navigation, no clicking, no DOM access.
- It **never mints or submits tokens** — the caller's browser harvests
  `g-recaptcha-response` / `h-captcha-response` after clicking.
- reCAPTCHA **v3** and Cloudflare **Turnstile** are **detect-only**. They are
  behavioral / proof-of-work challenges, not vision puzzles, so `detect_captcha` flags them
  with `llm_solvable:false` and the solver returns nothing for them.

## Install / build

Requires **Go 1.26**.

```sh
make build        # -> bin/captcha-solver-mcp
```

The binary is a single stdio MCP server with no external runtime dependencies beyond your
configured LLM endpoint.

## Configuration

Configured entirely via environment variables, **validated at startup (fail-fast)**.
Copy `.env.example` as a starting point.

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `CAPTCHA_LLM_PROVIDER` | yes | — | `anthropic` \| `openai` \| `openai-compatible` \| `gemini` |
| `CAPTCHA_LLM_BASE_URL` | for `openai-compatible` | provider default | Endpoint override (local / Ollama / OpenRouter / gateway) |
| `CAPTCHA_LLM_API_KEY` | yes\* | — | \*May be **empty** for a keyless local/gateway endpoint **when `BASE_URL` is set** |
| `CAPTCHA_LLM_MODEL` | yes | — | Vision-capable model id (see compatibility notes) |
| `CAPTCHA_LLM_AUDIO_MODEL` | no | uses `MODEL` | Separate audio/transcription model (e.g. `whisper-1`) |
| `CAPTCHA_LLM_TEMPERATURE` | no | `0` | |
| `CAPTCHA_LLM_MAX_TOKENS` | no | `1024` | |
| `CAPTCHA_LLM_TIMEOUT` | no | `60s` | Go duration string |
| `CAPTCHA_LOG_LEVEL` | no | `info` | |

> **Keyless gateways:** if you point `CAPTCHA_LLM_BASE_URL` at an OpenAI-compatible gateway
> that does not require auth, leave `CAPTCHA_LLM_API_KEY` empty — startup will accept it.

## Tools

All nine tools are registered in `internal/mcp/server.go`. Every image/audio input accepts
**base64**, a **file path**, an **http(s) URL**, or a **data-URI**.

| Tool | Inputs | Output |
|------|--------|--------|
| `detect_captcha` | `html?`, `url?`, `screenshot?` | `{detected:[{type,sitekey?,iframe_url?,notes?,llm_solvable}],summary}` — deterministic; v3/Turnstile → `llm_solvable:false` |
| `solve_text_captcha` | `image`, `hint?`, `charset?`, `length?`, `case_sensitive?` | `{text,confidence}` (OCR) |
| `solve_audio_captcha` | `audio`, `language?` | `{text,confidence}`; errors clearly if provider has no transcription |
| `solve_grid_captcha` | `screenshot`, `instruction`, `rows`, `cols`, `image_width?`, `image_height?`, `captcha_type?` | `{tiles,centroids?,confidence,recheck_recommended}` (reCAPTCHA v2 / hCaptcha) |
| `solve_rotation_captcha` | `screenshot`, `instruction?` | `{choice,rotations,confidence}` (FunCaptcha, best-effort) |
| `get_stealth_script` | `engine?` = `playwright`\|`cdp`\|`generic` | `{script,apply,included_evasions}` — JS init script + how to inject it |
| `generate_fingerprint` | `os?`, `browser?`, `locale?`, `seed?` | `{fingerprint,header_set}` — coherent identity + matching headers (seeded = stable) |
| `plan_interaction` | `actions`, `seed?` | humanized mouse/keyboard timeline (Bézier paths, jitter, cadence) |
| `solver_info` | — | configured provider/model + per-type capability flags |

- `solve_grid_captcha` returns `centroids` (pixel `{x,y}` click points) only when
  `image_width` + `image_height` are supplied; otherwise the caller maps tile indices to
  pixels itself.
- `solve_audio_captcha` requires an audio-capable provider. `anthropic` is **vision-only**
  here and will return a clear "unsupported" error — use `openai` or `gemini`
  (set `CAPTCHA_LLM_AUDIO_MODEL`).
- `plan_interaction` is designed to pair with grid centroids: feed it `click` actions at the
  tile centroids to produce human-looking pointer motion.

## Captcha-type matrix

| Captcha | Tool | Who does what |
|---------|------|---------------|
| Image-text (distorted characters) | `solve_text_captcha` | Brain reads the text |
| Audio | `solve_audio_captcha` | Brain transcribes (needs openai/gemini) |
| reCAPTCHA **v2** (image grid) | `solve_grid_captcha` | Brain returns tiles/centroids; **caller clicks + harvests token** |
| **hCaptcha** (image grid) | `solve_grid_captcha` | Brain returns tiles/centroids; **caller clicks + harvests token** |
| **FunCaptcha** (rotation) | `solve_rotation_captcha` | Brain picks rotation (best-effort); caller applies it |
| reCAPTCHA **v3** | `detect_captcha` only | **Detect-only** — behavioral, not LLM-solvable |
| Cloudflare **Turnstile** | `detect_captcha` only | **Detect-only** — PoW/behavioral, not LLM-solvable |

## Browser integration recipe (reCAPTCHA v2 brain/hands loop)

The server is the brain; your browser is the hands. The loop for an image-grid challenge:

1. **Prep identity** — call `generate_fingerprint` (with a stable `seed`) and apply the
   returned headers; call `get_stealth_script` and inject the `script` *before any page
   script runs* (`page.addInitScript` in Playwright, or
   `Page.addScriptToEvaluateOnNewDocument` over raw CDP).
2. **Trigger + screenshot** — open the challenge, screenshot the tile grid, and read the
   challenge text (e.g. "select all buses") and grid size (e.g. 3×3).
3. **Ask the brain** — `solve_grid_captcha` with the screenshot, instruction, `rows`, `cols`,
   and `image_width`/`image_height` to get `tiles` + `centroids`.
4. **Humanize** — feed the centroids as `click` actions to `plan_interaction`; replay the
   resulting timeline so the pointer moves on a Bézier path with realistic cadence.
5. **Verify** — click the verify button.
6. **Loop if dynamic** — if `recheck_recommended` is true (tiles re-render after each
   click), re-screenshot and repeat steps 3–5 until the grid clears.
7. **Harvest** — read the `g-recaptcha-response` token from the page and submit your form.

Runnable reference implementations are in [`examples/`](./examples).

## Usage

### Register with an MCP client

The server speaks MCP over **stdio**. Point your MCP client at the built binary:

```json
{
  "mcpServers": {
    "captcha-solver": {
      "command": "/absolute/path/to/bin/captcha-solver-mcp",
      "env": {
        "CAPTCHA_LLM_PROVIDER": "openai-compatible",
        "CAPTCHA_LLM_BASE_URL": "https://your-gateway.example.com/v1",
        "CAPTCHA_LLM_API_KEY": "sk-...",
        "CAPTCHA_LLM_MODEL": "openai/gpt-5.4"
      }
    }
  }
}
```

### Copy-pasteable env (vision model)

```sh
export CAPTCHA_LLM_PROVIDER=openai-compatible
export CAPTCHA_LLM_BASE_URL=https://your-gateway.example.com/v1
export CAPTCHA_LLM_API_KEY=sk-...
export CAPTCHA_LLM_MODEL=openai/gpt-5.4        # or zai/GLM-4.6V-FlashX, claude-opus-4-8, etc.
./bin/captcha-solver-mcp
```

## Testing

```sh
make test       # unit tests (go test ./...)
make e2e        # deterministic stdio end-to-end with a fake LLM (no network)
make e2e-live   # OPT-IN: live test against a real LLM gateway
```

The live e2e is gated behind `CAPTCHA_E2E_LIVE=1` and needs a real endpoint:

```sh
CAPTCHA_E2E_LIVE=1 \
CAPTCHA_LLM_PROVIDER=openai-compatible \
CAPTCHA_LLM_BASE_URL=https://your-gateway.example.com/v1 \
CAPTCHA_LLM_MODEL=openai/gpt-5.4 \
make e2e-live
```

## Compatibility notes

- **`max_tokens` vs `max_completion_tokens`:** the `openai-compatible` provider auto-retries
  with `max_completion_tokens` when a model (e.g. GPT-5.x) rejects `max_tokens`. This works
  transparently across old and new OpenAI-style backends — you don't configure anything.
- **Prefer a vision-capable model** for the image/grid tools (e.g. `openai/gpt-5.4`,
  `zai/GLM-4.6V-FlashX`). Reasoning models that emit `<think>…</think>` still work
  (responses are JSON-extracted), but non-reasoning vision models give cleaner output.
- **TLS/JA3 + HTTP/2 fingerprints are browser/proxy-level.** The stealth tools only
  *advise* on these — they cannot change them. To control them, terminate through a
  fingerprint-aware proxy or a browser build that emits the JA3/H2 profile you want.

## Roadmap / not yet built

Two pieces are **designed but deferred** (see
[`docs/superpowers/specs/`](./docs/superpowers/specs) and
[`docs/superpowers/plans/`](./docs/superpowers/plans)):

- **Deterministic browser e2e harness** — driving Playwright Chromium *and* Lightpanda
  against a local fake-captcha page, to exercise the full brain/hands loop in CI without
  hitting real sites.
- **Gemini provider** — a stub exists (`internal/llm/gemini.go`); the `generateContent`
  wire format still needs verification against current Google docs before it ships. Use
  `openai` / `openai-compatible` / `anthropic` in the meantime.
