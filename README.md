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
| `CAPTCHA_LLM_PROVIDER` | yes | — | `anthropic` \| `openai` \| `openai-compatible` \| `gemini` (all four fully implemented) |
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

> **Gemini:** the `gemini` provider is **fully implemented** against Google's native
> `generateContent` wire format (vision *and* audio) and is unit-tested. Live use needs a
> real Google Gemini API key (`generativelanguage.googleapis.com`) — the project's
> OpenAI-compatible test gateway can't exercise the native Gemini path, so it is covered by
> unit tests against a mock endpoint rather than the live e2e suite.

## Tools

All nine tools are registered in `internal/mcp/server.go`. Every image/audio input accepts
**base64**, a **file path**, an **http(s) URL**, or a **data-URI**.

| Tool | Inputs | Output |
|------|--------|--------|
| `detect_captcha` | `html?`, `url?`, `screenshot?` | `{detected:[{type,sitekey?,iframe_url?,notes?,llm_solvable}],summary}` — deterministic; v3/Turnstile → `llm_solvable:false` |
| `solve_text_captcha` | `image`, `hint?`, `charset?`, `length?`, `case_sensitive?`, `samples?`, `upscale?` | `{text,confidence}` (OCR) |
| `solve_audio_captcha` | `audio`, `language?` | `{text,confidence}`; errors clearly if provider has no transcription |
| `solve_grid_captcha` | `screenshot`, `instruction`, `rows`, `cols`, `image_width?`, `image_height?`, `captcha_type?`, `samples?`, `upscale?`, `annotate?` | `{tiles,centroids?,confidence,no_targets,recheck_recommended}` (reCAPTCHA v2 / hCaptcha) |
| `solve_rotation_captcha` | `screenshot`, `instruction?` | `{choice,rotations,confidence}` (FunCaptcha, best-effort) |
| `get_stealth_script` | `engine?` = `playwright`\|`cdp`\|`generic`, `os?`, `browser?`, `locale?`, `seed?` | `{script,apply,fingerprint,included_evasions}` — JS init script (templated to match `fingerprint`) + how to inject it |
| `generate_fingerprint` | `os?`, `browser?`, `locale?`, `seed?` | `{fingerprint,header_set}` — coherent identity + matching headers (seeded = stable) |
| `plan_interaction` | `actions`, `seed?` | humanized mouse/keyboard timeline (Bézier paths, jitter, cadence) |
| `solver_info` | — | configured provider/model + per-type capability flags |

- `solve_grid_captcha` returns `centroids` (pixel `{x,y}` click points) only when
  `image_width` + `image_height` are supplied; otherwise the caller maps tile indices to
  pixels itself.
- `solve_grid_captcha` new accuracy params: `samples` (run N times, majority-vote tiles;
  `samples:3` raises Pass@1 ~70% → Success@3 ~97%); `upscale` (1–6, upscale image before
  solving — helps small tiles); `annotate` (overlay numbered grid cells to reduce
  off-by-one errors). Output now includes `no_targets:true` when no tile matches — caller
  should click Verify instead of looping.
- `solve_text_captcha` new params: `samples` (majority-vote the transcription) and `upscale`
  (upscale image before OCR).
- `solve_audio_captcha` requires an audio-capable provider. `anthropic` is **vision-only**
  here and will return a clear "unsupported" error — use `openai` or `gemini`
  (set `CAPTCHA_LLM_AUDIO_MODEL`).
- `plan_interaction` is designed to pair with grid centroids: feed it `click` actions at the
  tile centroids to produce human-looking pointer motion.
- `get_stealth_script` accepts `os` / `browser` / `locale` / `seed` and returns a
  `fingerprint` field. The injected evasion script's WebGL vendor/renderer,
  `navigator.platform`, and `navigator.languages` are **templated to match that
  fingerprint** (coherent), so the script and the identity tell the same story. Inject the
  `script` and apply the returned `fingerprint` together — or call `get_stealth_script` and
  `generate_fingerprint` with the **same `seed`** to get an identical, coherent pair.

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

Runnable reference implementations are in [`examples/`](./examples):

- [`examples/playwright/solve-recaptcha.mjs`](./examples/playwright/solve-recaptcha.mjs) — image-grid path (vision model)
- [`examples/playwright/solve-recaptcha-audio.mjs`](./examples/playwright/solve-recaptcha-audio.mjs) — **audio path** (highest reliability; needs an audio-capable provider)

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

## Maximizing accuracy

### Image grids (reCAPTCHA v2 / hCaptcha)

- Use `samples: 3` — the server runs the vision model three times and majority-votes the
  tile selection. Published research (COGNITION, arXiv:2512.02318) shows this raises a ~70%
  Pass@1 model to ~97% Success@3. Cost: 3× LLM calls per round.
- Use `annotate: true` — overlays numbered cell labels on the grid image, which reduces
  off-by-one selection errors on dense or ambiguous grids.
- Use `upscale: 2` (or higher) for small tile grids — upscaling before the vision pass
  helps models resolve fine detail in compressed tiles.
- Use a strong vision model (GPT-5, Gemini-2.5, Claude Opus 4).
- Watch `no_targets` in the response — when `true`, no tile matches the instruction and you
  should click **Verify** rather than looping on an empty selection.
- Watch `recheck_recommended` — when `true` the grid is dynamic (tiles re-render after each
  click); re-screenshot and loop.

### reCAPTCHA v2 — audio route (highest reliability)

The **audio challenge is the most reliable programmatic route** for reCAPTCHA v2. It converts
to a speech-to-text problem that modern ASR models handle at 70–97% accuracy (vs ~70–80%
Pass@1 for image grids). See
[`examples/playwright/solve-recaptcha-audio.mjs`](./examples/playwright/solve-recaptcha-audio.mjs)
for a runnable annotated recipe.

Requires an audio-capable provider:

```sh
CAPTCHA_LLM_PROVIDER=openai
CAPTCHA_LLM_AUDIO_MODEL=whisper-1
# or
CAPTCHA_LLM_PROVIDER=gemini
CAPTCHA_LLM_AUDIO_MODEL=gemini-2.0-flash
```

`anthropic` is vision-only and will return a clear "unsupported" error.

### Distorted text captchas

- `samples: 3` + `upscale: 2` for heavily distorted or low-resolution images.
- Modern vision models (GPT-4o, Gemini-2.5) handle mild distortion zero-shot at ~99%
  without upscaling; reserve upscaling for genuinely blurry or tiny inputs.

### Cost note

`samples: N` = N× LLM calls per tool invocation. Use `samples: 1` (the default) when
latency or cost matters more than marginal accuracy gain.

## Accuracy expectations (from published research)

High solver accuracy does not guarantee a token. reCAPTCHA v2 and hCaptcha gate final token
minting on the caller's browser + IP behavioral score — a high recognition rate in isolation
does not compensate for headless-browser signals, suspicious IP reputation, or missing
cookies. Use a real browser profile (`CAPTCHA_E2E_HEADED=1`) for best odds.

| Captcha | Approach | Reported accuracy | Source |
|---------|----------|-------------------|--------|
| reCAPTCHA v2 image grid | fine-tuned YOLOv8 (not LLM) | ~100% image-solving | ETH Zurich "Breaking reCAPTCHAv2", arXiv:2409.08831 |
| Recognition grids (MLLM) | GPT-5/Gemini-2.5, optimized prompt | >80% Pass@1; ~97% Success@3 | COGNITION, arXiv:2512.02318 |
| Visual captchas (agentic VLM) | general solver | 60.7% controlled / 70.6% wild | Halligan, USENIX Security 2025 |
| reCAPTCHA v2 audio | speech-to-text (Whisper / Gemini) | ~90% (unCaptcha2); 70–97% modern ASR | unCaptcha2, USENIX WOOT |
| Distorted text | multimodal LLM + upscale | ~99% on mild distortion | modern VLM OCR benchmarks |

## Testing

```sh
make test         # unit tests (go test ./...)
make e2e          # deterministic stdio end-to-end with a fake LLM (no network)
make e2e-live     # OPT-IN: live test against a real LLM gateway
make e2e-browser  # OPT-IN: live Playwright browser e2e against the 2captcha demo
```

The live e2e is gated behind `CAPTCHA_E2E_LIVE=1` and needs a real endpoint:

```sh
CAPTCHA_E2E_LIVE=1 \
CAPTCHA_LLM_PROVIDER=openai-compatible \
CAPTCHA_LLM_BASE_URL=https://your-gateway.example.com/v1 \
CAPTCHA_LLM_MODEL=openai/gpt-5.4 \
make e2e-live
```

### Browser e2e (Playwright → 2captcha demo)

`make e2e-browser` drives a real Chromium (via Playwright) against the public
[2captcha demo](https://2captcha.com/demo) and solves captchas through the MCP server +
a real vision model. It is **network- and browser-gated** (build tags `e2e,live`,
`CAPTCHA_E2E_LIVE=1`, `node` + an installed Playwright Chromium); the offline `make e2e`
suite never runs it. Prereqs:

```sh
npm install
npx playwright install chromium
make e2e-browser
```

> Node deps are pinned by a single lockfile, **`package-lock.json`** (npm). Use `npm` to
> install so the lockfile stays canonical.

Two paths (`test/e2e/playwright/*.mjs`, wrapped by `test/e2e/browser_test.go`):

- **Text captcha (`solve-text.mjs`) — verified end-to-end.** Screenshots the demo's
  distorted-text image, calls `solve_text_captcha`, types the prediction, clicks *Check*,
  and asserts the demo's own *"Captcha is passed successfully!"* indicator (ground truth).
  Retries up to 2 fresh captchas before failing, since OCR is stochastic.
- **reCAPTCHA v2 (`solve-recaptcha-v2.mjs`) — best-effort.** Applies a stealth script +
  `generate_fingerprint` profile, clicks the checkbox, then loops `solve_grid_captcha` →
  `plan_interaction` → humanized clicks over the tile grid, and reads
  `g-recaptcha-response`. Real Google reCAPTCHA commonly resists headless automation, so
  this path **logs the outcome and exits 0 even with no token** — it fails only on a real
  script/transport error. Set **`CAPTCHA_E2E_HEADED=1`** to launch a visible, slow-mo
  browser for a real-profile attempt that improves the odds of Google minting a token.
  Headless reCAPTCHA v2 usually **won't** mint a token — that is inherent to Google's
  defenses, not a bug in this harness.

## Compatibility notes

- **`max_tokens` vs `max_completion_tokens`:** the `openai-compatible` provider auto-retries
  with `max_completion_tokens` when a model (e.g. GPT-5.x) rejects `max_tokens`. This works
  transparently across old and new OpenAI-style backends — you don't configure anything.
- **Prefer a vision-capable model** for the image/grid tools (e.g. `openai/gpt-5.4`,
  `zai/GLM-4.6V-FlashX`). Reasoning models that emit `<think>…</think>` (or
  `<thinking>…</thinking>`) are now **handled** — the scratchpad block is stripped *before*
  JSON extraction, so a JSON-looking object inside the reasoning never gets mistaken for the
  answer. Non-reasoning vision models still give the lowest latency.
- **TLS/JA3 + HTTP/2 fingerprints are browser/proxy-level.** The stealth tools only
  *advise* on these — they cannot change them. To control them, terminate through a
  fingerprint-aware proxy or a browser build that emits the JA3/H2 profile you want.

## Roadmap / not yet built

The **Gemini provider** and **WebGL/fingerprint coherence** are now **done** (see the
Provider and Tools sections above); they are no longer on this list.

One piece remains **designed but deferred** (see
[`docs/superpowers/specs/`](./docs/superpowers/specs) and
[`docs/superpowers/plans/`](./docs/superpowers/plans)):

- **YOLOv8 grid classifier** — a fine-tuned YOLOv8 classifier/segmenter (per ETH
  arXiv:2409.08831) embedded via an ONNX runtime would beat zero-shot LLM accuracy on
  standard reCAPTCHA grids (~100% image-solving vs ~80% Pass@1 for zero-shot MLLMs), at the
  cost of training data and a CV runtime dependency. Deferred as a separate effort from the
  LLM path.

- **Deterministic (offline) browser e2e harness** — driving Playwright Chromium *and*
  Lightpanda against a *local* fake-captcha page, to exercise the full brain/hands loop in
  CI without hitting real sites. A **live** browser harness already exists (`make
  e2e-browser`, see [Testing](#testing)): it targets the 2captcha demo, is
  network/browser-gated, verifies the text path end-to-end against the demo's success
  indicator, and runs the reCAPTCHA v2 path best-effort (with an optional
  `CAPTCHA_E2E_HEADED=1` real-profile attempt). The remaining work is the offline,
  deterministic local-page variant for CI.

> Note: headless reCAPTCHA v2 usually won't mint a token regardless of harness quality —
> that is Google's anti-automation defense working as intended, not a defect here.
