// Reference: solve a reCAPTCHA v2 challenge via the AUDIO path with Playwright as the
// "hands" and captcha-solver-mcp as the "brain".
//
// WHY AUDIO? The audio challenge converts to a speech-to-text problem, which modern ASR
// models handle at 70–97% accuracy (unCaptcha2 reports ~90% on the USENIX WOOT 2019 dataset;
// modern Whisper-class models typically land 80–97% depending on audio quality). This is
// generally higher than the zero-shot vision path on image grids (~70–80% Pass@1), making
// the audio route the most reliable programmatic path for reCAPTCHA v2.
//
// PROVIDER NOTE: `solve_audio_captcha` requires an audio-capable provider.
// Anthropic is vision-only and will return a clear "unsupported" error. Use one of:
//   CAPTCHA_LLM_PROVIDER=openai   +  CAPTCHA_LLM_AUDIO_MODEL=whisper-1
//   CAPTCHA_LLM_PROVIDER=gemini   +  CAPTCHA_LLM_AUDIO_MODEL=gemini-2.0-flash  (or similar)
//
// Run it yourself (this repo does not vendor node_modules):
//   npm i playwright
//   node examples/playwright/solve-recaptcha-audio.mjs
//
// Required env:
//   CAPTCHA_BIN=/abs/path/to/bin/captcha-solver-mcp
//   CAPTCHA_LLM_PROVIDER=openai          # or gemini — NOT anthropic
//   CAPTCHA_LLM_API_KEY=sk-...
//   CAPTCHA_LLM_MODEL=gpt-4o             # vision model for general use
//   CAPTCHA_LLM_AUDIO_MODEL=whisper-1    # transcription model
//
// HONEST CAVEAT: Google's reCAPTCHA infrastructure tracks behavioral signals beyond what
// this script can control (TLS fingerprint, IP reputation, headless-browser detection). High
// ASR accuracy does NOT guarantee a token — especially from headless automation.
// Set CAPTCHA_E2E_HEADED=1 to launch a visible browser for a real-profile attempt that
// significantly improves the odds. Headless reCAPTCHA v2 may never mint a token regardless
// of solver accuracy; that is Google's anti-automation defense working as intended.
//
// AUTHORIZED USE ONLY. Run this against captcha integrations you own or are permitted to
// test. Do not use it to evade protections on sites you do not control.

import { chromium } from 'playwright';
import { McpStdioClient } from '../mcp-client.mjs';

const TARGET_URL = 'https://2captcha.com/demo/recaptcha-v2';
const CAPTCHA_BIN = process.env.CAPTCHA_BIN || '../../bin/captcha-solver-mcp';
const HEADED = process.env.CAPTCHA_E2E_HEADED === '1';

// ── Step 1: Spawn the MCP server ───────────────────────────────────────────────
// Pass LLM config from env straight through. The server validates on startup.
const serverEnv = {
  CAPTCHA_LLM_PROVIDER:     process.env.CAPTCHA_LLM_PROVIDER,
  CAPTCHA_LLM_BASE_URL:     process.env.CAPTCHA_LLM_BASE_URL,
  CAPTCHA_LLM_API_KEY:      process.env.CAPTCHA_LLM_API_KEY,
  CAPTCHA_LLM_MODEL:        process.env.CAPTCHA_LLM_MODEL,
  // whisper-1 (openai) or a Gemini model — the server routes audio to this separately.
  CAPTCHA_LLM_AUDIO_MODEL:  process.env.CAPTCHA_LLM_AUDIO_MODEL,
};

const mcp = new McpStdioClient(CAPTCHA_BIN, serverEnv);
await mcp.initialize();

// Verify the configured provider supports audio before we start a browser.
const info = await mcp.callTool('solver_info', {});
if (!info.capabilities?.audio) {
  console.error(
    `[error] provider "${info.provider}" has no audio capability. ` +
    'Set CAPTCHA_LLM_PROVIDER=openai or gemini and CAPTCHA_LLM_AUDIO_MODEL.',
  );
  mcp.close();
  process.exit(1);
}
console.log(`[mcp] provider=${info.provider} audio_model=${serverEnv.CAPTCHA_LLM_AUDIO_MODEL ?? info.model}`);

// ── Step 1b: Build a coherent stealth identity ─────────────────────────────────
// generate_fingerprint + get_stealth_script with the SAME seed give a coherent pair:
// the evasion script's WebGL/platform strings match the header set exactly.
const fp = await mcp.callTool('generate_fingerprint', {
  os: 'macos', browser: 'chrome', locale: 'en-US', seed: 42,
});
const stealth = await mcp.callTool('get_stealth_script', {
  engine: 'playwright', os: 'macos', browser: 'chrome', locale: 'en-US', seed: 42,
});
console.log('[mcp] stealth evasions:', stealth.included_evasions);

// ── Launch Chromium ────────────────────────────────────────────────────────────
const browser = await chromium.launch({ headless: !HEADED, slowMo: HEADED ? 200 : 0 });
const context = await browser.newContext({
  userAgent: fp.header_set['User-Agent'] || fp.header_set['user-agent'],
  locale:    'en-US',
  extraHTTPHeaders: fp.header_set,
});
// Inject the evasion script BEFORE any page script runs — this is the correct order.
await context.addInitScript({ content: stealth.script });

const page = await context.newPage();

// ── Step 2: Navigate and click the "I'm not a robot" checkbox ─────────────────
// The checkbox lives inside the reCAPTCHA anchor iframe.
console.log('[browser] navigating to', TARGET_URL);
await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' });

// NOTE: Selector `iframe[title="reCAPTCHA"]` is the anchor (checkbox) iframe.
// The exact title varies slightly by locale; adjust if needed.
const anchorFrame = page.frameLocator('iframe[title="reCAPTCHA"]');
await anchorFrame.locator('#recaptcha-anchor').click();
console.log('[browser] clicked checkbox, waiting for challenge iframe...');

// The challenge iframe appears after Google evaluates the initial click signal.
// Its title contains "challenge" (e.g. "recaptcha challenge expires in two minutes").
const challengeFrame = page.frameLocator('iframe[title*="challenge"]');

// Wait for the challenge to load (image grid OR audio button present).
await challengeFrame.locator('#recaptcha-audio-button, .rc-imageselect-challenge').waitFor({ timeout: 15_000 });

// ── Step 3: Switch to the audio challenge ─────────────────────────────────────
// If an image grid is shown first, click the audio button to switch.
// The audio button selector is stable across reCAPTCHA versions.
const audioBtn = challengeFrame.locator('#recaptcha-audio-button');
const audioBtnVisible = await audioBtn.isVisible().catch(() => false);
if (audioBtnVisible) {
  await audioBtn.click();
  console.log('[browser] switched to audio challenge');
  // Wait for the audio player to appear.
  await challengeFrame.locator('.rc-audiochallenge-tdownload-link, #audio-source').waitFor({ timeout: 10_000 });
} else {
  // We might already be on the audio challenge (unlikely on first load, but handle it).
  console.log('[browser] audio challenge already visible');
}

// ── Step 4: Read the audio MP3 URL ────────────────────────────────────────────
// Two selectors cover different reCAPTCHA versions:
//   .rc-audiochallenge-tdownload-link  — the "Download" link (href = MP3 URL)
//   #audio-source                      — the <source> element inside <audio>
// We try the download link first (more reliable); fall back to audio-source src.
//
// NOTE: Google periodically changes these selectors. Treat them as illustrative.
let audioUrl = null;

const downloadLink = challengeFrame.locator('.rc-audiochallenge-tdownload-link');
if (await downloadLink.isVisible().catch(() => false)) {
  audioUrl = await downloadLink.getAttribute('href');
}

if (!audioUrl) {
  const audioSource = challengeFrame.locator('#audio-source');
  if (await audioSource.isVisible().catch(() => false)) {
    audioUrl = await audioSource.getAttribute('src');
  }
}

if (!audioUrl) {
  console.warn('[warn] could not locate audio URL — Google may have blocked the audio endpoint.');
  console.warn('       Try CAPTCHA_E2E_HEADED=1 for a real-profile attempt.');
  mcp.close();
  await browser.close();
  process.exit(0); // non-fatal: document the outcome, don't throw
}
console.log('[browser] audio URL:', audioUrl);

// ── Step 5: Pass the audio URL to solve_audio_captcha ─────────────────────────
// imageutil.Decode on the server accepts http(s) URLs directly — no need to fetch
// and base64-encode on our side. The server downloads and decodes it.
//
// If the MP3 URL requires a session cookie to download, you would instead:
//   const resp = await fetch(audioUrl, { headers: { Cookie: ... } });
//   const buf = Buffer.from(await resp.arrayBuffer());
//   const audioInput = `data:audio/mpeg;base64,${buf.toString('base64')}`;
// For the public reCAPTCHA demo the URL is publicly fetchable without auth.
console.log('[mcp] calling solve_audio_captcha...');
let solveResult;
try {
  solveResult = await mcp.callTool('solve_audio_captcha', {
    audio: audioUrl,
    language: 'en',
  });
} catch (err) {
  console.error('[error] solve_audio_captcha failed:', err.message);
  mcp.close();
  await browser.close();
  process.exit(0); // non-fatal exit so CI doesn't fail on provider misconfiguration
}

const transcription = solveResult.text ?? '';
const confidence    = solveResult.confidence ?? 0;
console.log(`[mcp] transcription="${transcription}" confidence=${confidence.toFixed(2)}`);

if (!transcription) {
  console.warn('[warn] empty transcription — cannot proceed');
  mcp.close();
  await browser.close();
  process.exit(0);
}

// ── Step 6: Type the answer and click Verify ──────────────────────────────────
// #audio-response is the text input inside the challenge iframe.
// #recaptcha-verify-button submits the answer.
await challengeFrame.locator('#audio-response').fill(transcription);
await challengeFrame.locator('#recaptcha-verify-button').click();
console.log('[browser] submitted answer, waiting for response...');

// Give Google a moment to evaluate the submission.
await page.waitForTimeout(2000);

// ── Step 7: Read the token ────────────────────────────────────────────────────
// The caller (us) reads the token — the MCP server never sees it.
// g-recaptcha-response is a hidden textarea injected into the HOST page (not the iframe).
const token = await page.evaluate(() => {
  const el = document.querySelector('textarea[name="g-recaptcha-response"]');
  return el ? el.value : '';
});

if (token) {
  console.log(`[result] token obtained (length=${token.length}) — challenge passed.`);
} else {
  console.warn(
    '[result] no token. Possible reasons:\n' +
    '  • Google rejected the audio answer (try again or use a better audio model)\n' +
    '  • Headless behavioral signals flagged the session (try CAPTCHA_E2E_HEADED=1)\n' +
    '  • The audio endpoint was blocked before we fetched the MP3\n' +
    '  • Google served an image challenge even after clicking the audio button\n' +
    'This is expected in many headless environments — not a bug in the solver.',
  );
}

mcp.close();
await browser.close();
