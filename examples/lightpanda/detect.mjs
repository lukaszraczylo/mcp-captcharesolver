// Reference: detect captchas on a page using Lightpanda (a lightweight headless browser)
// as the "hands" and captcha-solver-mcp's deterministic `detect_captcha` as the "brain".
//
// `detect_captcha` is a pure, deterministic HTML scan — it needs no LLM call to classify
// known providers. It flags reCAPTCHA v3 and Cloudflare Turnstile as `llm_solvable:false`
// because those are behavioral / proof-of-work challenges, not vision puzzles.
//
// This example shows the simplest possible integration: fetch the page DOM with Lightpanda,
// hand the HTML to the server, and print what was found. No clicking, no tokens.
//
// Run it yourself (this repo does not vendor node_modules):
//   npm i playwright          # used here purely as a CDP client to talk to Lightpanda
//   lightpanda serve --host 127.0.0.1 --port 9222   # start Lightpanda's CDP endpoint
//   node examples/lightpanda/detect.mjs https://your-own-test-page.example
//
// Required env:
//   CAPTCHA_BIN=/abs/path/to/bin/captcha-solver-mcp
//   CAPTCHA_LLM_PROVIDER=anthropic            # any valid provider; detection won't call it
//   CAPTCHA_LLM_API_KEY=sk-...
//   CAPTCHA_LLM_MODEL=claude-opus-4-8
//   LIGHTPANDA_CDP=ws://127.0.0.1:9222        # Lightpanda CDP websocket (optional override)
//
// AUTHORIZED USE ONLY. Detect captchas on pages you own or are permitted to test.

import { chromium } from 'playwright';
import { McpStdioClient } from '../mcp-client.mjs';

const TARGET_URL = process.argv[2];
const CAPTCHA_BIN = process.env.CAPTCHA_BIN || '../../bin/captcha-solver-mcp';
const LIGHTPANDA_CDP = process.env.LIGHTPANDA_CDP || 'ws://127.0.0.1:9222';

if (!TARGET_URL) {
  console.error('usage: node detect.mjs <url-of-your-own-test-page>');
  process.exit(2);
}

// 1. Start the brain. detect_captcha is deterministic, but the server still validates its
//    LLM config at startup, so the CAPTCHA_LLM_* vars must be present and valid.
const mcp = new McpStdioClient(CAPTCHA_BIN, {
  CAPTCHA_LLM_PROVIDER: process.env.CAPTCHA_LLM_PROVIDER,
  CAPTCHA_LLM_BASE_URL: process.env.CAPTCHA_LLM_BASE_URL,
  CAPTCHA_LLM_API_KEY: process.env.CAPTCHA_LLM_API_KEY,
  CAPTCHA_LLM_MODEL: process.env.CAPTCHA_LLM_MODEL,
});
await mcp.initialize();

// 2. Connect to Lightpanda over CDP and fetch the rendered DOM.
//    (Lightpanda speaks the Chrome DevTools Protocol, so any CDP client works; we reuse
//     Playwright's connectOverCDP here to keep the example dependency-light.)
const browser = await chromium.connectOverCDP(LIGHTPANDA_CDP);
const context = browser.contexts()[0] || (await browser.newContext());
const page = context.pages()[0] || (await context.newPage());

await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' });
const html = await page.content(); // full serialized DOM, including captcha iframes/markers

// 3. Ask the brain to classify. Pure HTML scan — no LLM round-trip.
const result = await mcp.callTool('detect_captcha', { url: TARGET_URL, html });

console.log(result.summary);
for (const d of result.detected) {
  const verdict = d.llm_solvable ? 'LLM-solvable' : 'DETECT-ONLY (behavioral/PoW)';
  console.log(`- ${d.type}${d.sitekey ? ` sitekey=${d.sitekey}` : ''} -> ${verdict}`);
  if (d.notes) console.log(`    notes: ${d.notes}`);
}

if (result.detected.length === 0) {
  console.log('no known captcha provider found in the DOM');
}

mcp.close();
await browser.close();
