# captcha-solver-mcp — Design Spec

- **Date:** 2026-06-07
- **Status:** Approved (design); pending implementation plan
- **Language/runtime:** Go
- **Transport:** MCP over stdio

## 1. Overview

A Go MCP server that acts as a **vision/LLM "brain"** for captcha solving and a
**stealth-artifact generator** for browser automation. It is **solver-only and
browser-agnostic**: callers (Playwright, Lightpanda, or any CDP/DOM automation)
feed it images, audio, challenge screenshots, and page context; it returns
answers, tile-click decisions, and stealth artifacts. The caller's browser is
the "hands" — it performs clicks/typing and harvests any tokens.

### Goals
- Solve, via a configurable LLM, the captcha classes an LLM *can* genuinely
  solve: distorted-text images, audio challenges, and image-grid challenges
  (reCAPTCHA v2, hCaptcha) by returning which tiles to click.
- Detect captcha presence/type deterministically from page HTML, with an
  LLM-vision fallback from a screenshot.
- Emit stealth/anti-bot artifacts the caller injects/applies: evasion init
  scripts, coherent fingerprint profiles, humanized interaction plans, and
  matching header/client-hint sets.
- Be fully unit-testable offline (LLM behind an interface, fake client) and
  covered by deterministic e2e plus an opt-in live e2e tier.

### Non-goals
- Driving a browser itself (no embedded CDP/chromedp).
- Minting cryptographic tokens (`g-recaptcha-response`, hCaptcha/Turnstile
  tokens). The MCP never forges tokens.
- "Solving" reCAPTCHA v3 / Cloudflare Turnstile — these are behavioral /
  proof-of-work score systems with no visual challenge; an LLM cannot solve
  them. They are **detect-only**.
- Changing the browser's TLS/JA3 or HTTP/2 fingerprint (browser/proxy-level).
  The MCP only *advises* on these.
- Third-party paid solver APIs (2Captcha/CapSolver/etc.). A provider seam is
  left open but no such provider is built.

## 2. Authorized-use & ethics

This is a dual-use automation/QA tool. Intended use: testing one's own
anti-bot defenses and captcha integrations, accessibility automation, and
sanctioned scraping/testing. It is **not** for evading protections on sites the
operator does not own/control, fraud, or mass abuse. The README states this
plainly. No technical "guardrails" that would break legitimate use are added;
the constraint is policy, not code.

## 3. Architecture

```
                         ┌──────────────────────────────────────────┐
 caller's browser        │            captcha-solver-mcp (Go)        │
 (Playwright / Lightpanda)│                                          │
        │                │  internal/mcp     tool registration       │
   screenshot/audio/html │     │                                     │
        ├───────────────►│  internal/captcha  detect/text/audio/     │
        │                │     │              grid/rotation           │
        │                │  internal/stealth  scripts/fingerprint/    │
        │   answers,     │     │              interaction/headers     │
        │   tile picks,  │  internal/llm      Vision()/Transcribe()   │
        │   stealth      │     │              (interface)             │
        │◄───────────────┤     ├── openai.go / anthropic.go / gemini  │
   clicks/inject/apply   │  internal/imageutil  decode/crop/slice     │
   harvest token         │  internal/config     env parse/validate    │
                         └──────────────────────────────────────────┘
```

The LLM is hidden behind an interface so all handlers are testable with a fake
client (no network). The core (detection, image utils, stealth generation,
interaction planning) is pure Go and deterministic.

## 4. Project layout

```
cmd/captcha-solver-mcp/main.go     # load config, wire MCP server, stdio transport
internal/config/                   # env parsing + validation, fail-fast
internal/llm/
  client.go                        # interface: Vision(ctx, imgs, prompt) / Transcribe(ctx, audio)
  openai.go                        # OpenAI-compatible (OpenAI, OpenRouter, Ollama, local)
  anthropic.go                     # Anthropic Messages API (vision)
  gemini.go                        # Gemini (vision + audio)
  fake.go                          # deterministic test double
internal/captcha/
  detect.go  text.go  audio.go  grid.go  rotation.go  types.go
internal/stealth/
  script.go  fingerprint.go  interaction.go  headers.go  types.go
  assets/                          # vendored evasion JS (sourced from current OSS evasions)
internal/imageutil/                # decode base64|path|url|data-uri, crop, grid-slice
internal/mcp/                      # tool registration + handlers
test/e2e/                          # deterministic + gated-live e2e (Playwright + Lightpanda)
  testpage/                        # local fake-captcha static page(s)
examples/                          # runnable Playwright (TS) + Lightpanda recipes
go.mod  README.md  .env.example  Makefile
```

## 5. LLM provider abstraction & env config

Interface (conceptual):

```go
type Client interface {
    Vision(ctx context.Context, images [][]byte, prompt string, opts Options) (string, error)
    Transcribe(ctx context.Context, audio []byte, opts Options) (string, error)
}
```

Providers selected by env. **Exact request/response shapes for each provider
are verified against current API docs (via context7) at implementation time —
not memorized.** Providers: `anthropic`, `openai`,
`openai-compatible`, `gemini`. At least one audio-capable provider
(`openai` transcription or `gemini`) is required for `solve_audio_captcha`;
`anthropic` is vision-only here.

Env vars:

| Var | Meaning | Default |
|---|---|---|
| `CAPTCHA_LLM_PROVIDER` | `anthropic` \| `openai` \| `openai-compatible` \| `gemini` | required |
| `CAPTCHA_LLM_BASE_URL` | endpoint override (local/Ollama/OpenRouter) | provider default |
| `CAPTCHA_LLM_API_KEY` | API key | required (except keyless local) |
| `CAPTCHA_LLM_MODEL` | vision-capable model id | required |
| `CAPTCHA_LLM_AUDIO_MODEL` | separate audio/transcription model | falls back to MODEL |
| `CAPTCHA_LLM_TEMPERATURE` | sampling temperature | `0` |
| `CAPTCHA_LLM_MAX_TOKENS` | response cap | provider default |
| `CAPTCHA_LLM_TIMEOUT` | per-call timeout | `60s` |
| `CAPTCHA_LOG_LEVEL` | `debug`\|`info`\|`warn`\|`error` | `info` |
| `CAPTCHA_E2E_LIVE` | enable gated live e2e | unset (off) |

Config is validated at startup; missing key/model fails fast with a clear
message. Audio support depends on provider capability — if the configured
provider/model cannot transcribe, `solve_audio_captcha` returns a clear
"unsupported by configured provider" error (not a guess).

## 6. MCP tool contracts

All image/audio inputs accept **base64 / local file path / URL / data-URI**.
All solving tools return a `confidence` field and may include raw model output
for transparency. Tools are **stateless and idempotent**.

### 6.1 `detect_captcha`
- **In:** `html?` (string), `url?` (string), `screenshot?` (base64).
- **Out:** `{ detected: [{ type, sitekey?, iframe_url?, llm_solvable, notes }], summary }`.
- **Behavior:** deterministic HTML heuristics first (sitekey attributes,
  known iframe hosts: `google.com/recaptcha`, `hcaptcha.com`,
  `challenges.cloudflare.com`, Arkose/FunCaptcha markers). If only a screenshot
  is provided or HTML is inconclusive, fall back to LLM vision. v3/Turnstile are
  reported with `llm_solvable: false`.

### 6.2 `solve_text_captcha`
- **In:** `image`, `hint?`, `charset?`, `length?`, `case_sensitive?`.
- **Out:** `{ text, confidence }`.
- Distorted-text OCR via LLM vision.

### 6.3 `solve_audio_captcha`
- **In:** `audio`, `language?`.
- **Out:** `{ text, confidence }`.
- Requires audio-capable provider/model; else structured "unsupported" error.

### 6.4 `solve_grid_captcha` (reCAPTCHA v2 + hCaptcha)
- **In:** `screenshot` (base64), `instruction` (string),
  `grid {rows, cols}` *or* `tiles [{ index, bbox }]`,
  `captcha_type?` (`recaptcha`|`hcaptcha`), `dynamic?` (bool).
- **Out:** `{ tiles_to_click: [index...], centroids?: [{x,y}...], confidence, recheck_recommended }`.
- LLM vision picks matching tiles. `centroids` returned when a uniform grid or
  per-tile bboxes are supplied, to feed humanized clicking. For dynamic grids
  the caller re-screenshots and calls again until the token field populates;
  `recheck_recommended` hints when another round is likely needed.

### 6.5 `solve_rotation_captcha` (FunCaptcha/Arkose — best-effort)
- **In:** `images`/`screenshot`, `instruction?`.
- **Out:** `{ choice | rotations, confidence }`. Marked best-effort.

### 6.6 `solver_info`
- **In:** —.
- **Out:** configured provider/model + per-captcha-type support flags +
  stealth capabilities available. Health/introspection.

### 6.7 `get_stealth_script` (stealth)
- **In:** `engine?` (`playwright`|`cdp`|`generic`), `options?` (toggle individual evasions).
- **Out:** `{ script, apply: { how, api }, included_evasions: [...] }`.
- Returns a JS init script (vendored from current OSS evasions) plus
  apply instructions (`addInitScript` / `Page.addScriptToEvaluateOnNewDocument`).

### 6.8 `generate_fingerprint` (stealth)
- **In:** `os?`, `browser?`, `locale?`, `seed?`.
- **Out:** coherent profile `{ user_agent, sec_ch_ua, viewport, locale,
  timezone, platform, webgl {vendor, renderer}, hardware_concurrency,
  device_memory, ... }` + per-engine apply snippets. `seed` makes an identity
  stable across runs. Internal consistency is enforced (UA ↔ platform ↔
  client-hints ↔ WebGL all agree).

### 6.9 `plan_interaction` (stealth)
- **In:** `actions` (e.g. tile centroids to click, elements to type into,
  scroll targets), `viewport?`, `seed?`.
- **Out:** humanized timeline: mouse Bézier paths with overshoot/jitter,
  Fitts-law-based timing, click dwell, typing cadence, scroll steps. Designed to
  pair with `solve_grid_captcha` output so clicks look human.

### 6.10 Header/client-hint sets
Delivered as part of `generate_fingerprint` output (matching HTTP headers +
`Accept-Language` consistent with the profile) plus advisory notes on TLS/JA3
and proxy, explicitly flagged as outside the MCP's direct control.

## 7. Captcha-type handling matrix

| Type | LLM-solvable? | MCP role |
|---|---|---|
| Image/distorted-text | Yes (direct value) | `solve_text_captcha` |
| Audio | Yes (if provider supports) | `solve_audio_captcha` |
| reCAPTCHA v2 | Yes (tile picks; caller clicks + harvests token) | `solve_grid_captcha` |
| hCaptcha | Yes (tile picks; caller clicks + harvests token) | `solve_grid_captcha` |
| FunCaptcha/Arkose | Best-effort (rotation/choice) | `solve_rotation_captcha` |
| reCAPTCHA v3 | No | `detect_captcha` only (`llm_solvable:false`) |
| Cloudflare Turnstile | No | `detect_captcha` only (`llm_solvable:false`) |

## 8. Stealth subsystem (all four capabilities)

1. **Evasion init scripts** (`get_stealth_script`): patch `navigator.webdriver`,
   `chrome.runtime`, plugins/mimeTypes, `navigator.languages`, WebGL
   vendor/renderer, canvas noise, permissions API, `iframe.contentWindow`,
   window dimensions, etc. JS sourced/vendored from current open-source
   evasions, embedded as assets.
2. **Fingerprint profile generator** (`generate_fingerprint`): coherent,
   internally-consistent identity, optionally seeded for stability.
3. **Humanized interaction plans** (`plan_interaction`): Bézier mouse paths,
   Fitts-law timing, jitter/overshoot, typing cadence, scroll — the largest
   lever on reCAPTCHA v2 score; pairs with grid picks.
4. **Header/client-hint sets + advisories**: consistent headers; TLS/JA3 +
   proxy notes marked advisory (out of MCP control).

## 9. Browser integration recipes (`examples/`)

**reCAPTCHA v2 loop (caller side):**
1. Apply `generate_fingerprint` to the context; inject `get_stealth_script`.
2. Screenshot the challenge iframe + read instruction text.
3. → `solve_grid_captcha{ screenshot, instruction, grid:{rows:3,cols:3} }`.
4. → `plan_interaction{ actions: centroids }`; perform humanized clicks; click Verify.
5. If grid reloads (`dynamic`), re-screenshot → repeat; else read
   `g-recaptcha-response` token.

Runnable examples shipped for **Playwright (TS)** and **Lightpanda**.

## 10. Error handling & confidence

- Unparseable LLM output → one stricter-prompt retry, then a structured error.
  Never fabricate an answer.
- Every solve returns `confidence`; raw model text optionally included.
- Provider/transport errors surfaced with actionable messages.
- Invalid/undecodable media → clear input error.

## 11. Testing strategy

### Unit (offline, default)
- Detection: pure, fixture-driven (HTML samples per provider).
- imageutil: decode/crop/grid-slice with image fixtures.
- Stealth: fingerprint internal-consistency assertions; interaction-plan
  invariants (monotonic time, paths within viewport, endpoints hit targets);
  script generation snapshot.
- Handlers: fake LLM client → deterministic.
- Table-driven; golangci clean incl. fieldalignment; gosec incl. `_test.go`.

### Deterministic e2e (default, CI-safe, offline)
- Launch the **real MCP server binary over stdio** and exercise every tool
  through the real MCP protocol.
- Drive a **real browser against a LOCAL fake-captcha test page** (served from
  `test/e2e/testpage/`) using a **fake LLM** provider, running the full
  screenshot→solve→click→token loop with deterministic outcomes.
- **Both engines:** Playwright (Chromium) as the canonical grid-loop engine;
  Lightpanda for detect/DOM e2e.
- Stealth e2e: inject the script into the test page, assert
  `navigator.webdriver === undefined` and profile consistency.

### Gated live e2e (opt-in)
- Enabled only when `CAPTCHA_E2E_LIVE=1` and real LLM creds are present.
- Runs against real reCAPTCHA/hCaptcha demo pages with a real LLM + real
  browser. Skipped by default. Never required for a green build.

No GitHub Actions (build/test locally per project convention).

## 12. Build & quality gates
- `go build` / `go test ./...` locally; `Makefile` targets for build, test,
  e2e, e2e-live, lint.
- golangci-lint clean (incl. fieldalignment via `-fix`, never disabling
  linters); gosec applies to `_test.go` too.
- MCP Go SDK selection (`github.com/modelcontextprotocol/go-sdk` vs
  `github.com/mark3labs/mcp-go`) decided at implementation after checking
  current docs — not assumed here.

## 13. Out of scope
Self-driving browser; token minting; v3/Turnstile solving; TLS/JA3 rewriting;
paid third-party solver providers; stealth that requires browser-process control.

## 14. Open implementation items to verify (grounding)
- Go MCP SDK choice + current API (context7).
- Each LLM provider's vision + audio request/response shape (context7 / provider docs).
- Source + license of vendored evasion JS assets.
- Lightpanda screenshot fidelity for canvas-based grids (may limit it to
  detect/DOM e2e only).
