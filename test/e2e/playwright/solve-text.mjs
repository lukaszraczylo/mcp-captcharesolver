// RELIABLE browser e2e: solve the LIVE 2captcha text-captcha demo end-to-end.
//
//   server (brain) ── solve_text_captcha ──┐
//   Playwright (hands) ── screenshot/type/submit ──> 2captcha demo verifies the answer
//
// The 2captcha demo at https://2captcha.com/demo/normal is the ground truth: it renders a
// distorted-text image, accepts a typed answer, and shows a success message only when the
// answer is correct. We screenshot the image element, ask the MCP server's vision OCR to
// read it, type the prediction, click "Check", and assert the demo's own success indicator.
//
// AUTHORIZED USE ONLY — this targets 2captcha's public, self-verifying demo page.
//
// Required env (passed straight through to the spawned server):
//   CAPTCHA_LLM_PROVIDER=openai-compatible
//   CAPTCHA_LLM_BASE_URL=https://llmgw.h.raczylo.com
//   CAPTCHA_LLM_MODEL=openai/gpt-5.4
//   CAPTCHA_LLM_TIMEOUT=120s        (optional; defaults to 120s here)
//   CAPTCHA_LLM_API_KEY=...         (optional; gateway is keyless)
//   CAPTCHA_BIN=/abs/path           (optional; defaults to repo bin/captcha-solver-mcp)

import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { chromium } from 'playwright';
import { McpStdioClient } from '../../../examples/mcp-client.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, '../../..');

const TARGET_URL = 'https://2captcha.com/demo/normal';
const CAPTCHA_BIN = process.env.CAPTCHA_BIN || path.join(REPO_ROOT, 'bin', 'captcha-solver-mcp');
const HEADLESS = process.env.CAPTCHA_E2E_HEADLESS !== '0';
const MAX_ATTEMPTS = 3; // initial + up to 2 fresh retries; OCR is stochastic.

// Selectors verified against the live DOM (CSS-module hashes are partial-matched).
const SEL = {
  image: 'img[class*="_captchaImage"]',
  input: '#simple-captcha-field',
  submit: 'form[class*="_widgetForm"] button[type="submit"]',
  success: '[class*="_successMessage"]',
  error: '[class*="_errorMessage"], [class*="_alertError"]',
};

const serverEnv = {
  CAPTCHA_LLM_PROVIDER: process.env.CAPTCHA_LLM_PROVIDER || 'openai-compatible',
  CAPTCHA_LLM_BASE_URL: process.env.CAPTCHA_LLM_BASE_URL,
  CAPTCHA_LLM_API_KEY: process.env.CAPTCHA_LLM_API_KEY,
  CAPTCHA_LLM_MODEL: process.env.CAPTCHA_LLM_MODEL,
  CAPTCHA_LLM_TIMEOUT: process.env.CAPTCHA_LLM_TIMEOUT || '120s',
};

function log(...args) {
  console.log('[solve-text]', ...args);
}

async function main() {
  const mcp = new McpStdioClient(CAPTCHA_BIN, serverEnv);
  await mcp.initialize();

  // Stealth init script the page never sees as automation.
  const stealth = await mcp.callTool('get_stealth_script', { engine: 'playwright' });
  log('stealth evasions:', stealth.included_evasions);

  const browser = await chromium.launch({ headless: HEADLESS });
  const context = await browser.newContext();
  await context.addInitScript({ content: stealth.script });
  const page = await context.newPage();

  let passed = false;
  const predictions = [];

  try {
    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
      // Fresh load each attempt so any per-load captcha state resets.
      await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded', timeout: 60000 });

      const img = page.locator(SEL.image).first();
      await img.waitFor({ state: 'visible', timeout: 30000 });

      // Screenshot just the image element -> base64 PNG for the OCR tool.
      const pngBuf = await img.screenshot({ type: 'png' });
      const dataUri = `data:image/png;base64,${pngBuf.toString('base64')}`;

      // The demo's normal captcha is a 5-char alphanumeric, case-insensitive.
      const solved = await mcp.callTool('solve_text_captcha', {
        image: dataUri,
        length: 5,
        case_sensitive: false,
      });
      const prediction = (solved.text || '').trim();
      predictions.push(prediction);
      log(`attempt ${attempt}/${MAX_ATTEMPTS}: predicted="${prediction}" confidence=${solved.confidence}`);

      if (!prediction) {
        log(`attempt ${attempt}: empty prediction; retrying`);
        continue;
      }

      const input = page.locator(SEL.input);
      await input.fill('');
      await input.fill(prediction);
      await page.locator(SEL.submit).click();

      // Wait for either the success or the error indicator to appear.
      const outcome = await Promise.race([
        page
          .locator(SEL.success)
          .first()
          .waitFor({ state: 'visible', timeout: 15000 })
          .then(() => 'success')
          .catch(() => null),
        page
          .locator(SEL.error)
          .first()
          .waitFor({ state: 'visible', timeout: 15000 })
          .then(() => 'error')
          .catch(() => null),
      ]);

      if (outcome === 'success') {
        const msg = (await page.locator(SEL.success).first().innerText()).trim();
        log(`attempt ${attempt}: SUCCESS — demo says: "${msg}"`);
        passed = true;
        break;
      }

      const errText = (await page.locator(SEL.error).first().innerText().catch(() => '')).trim();
      log(`attempt ${attempt}: NOT accepted (demo: "${errText || 'no success indicator'}")`);
    }
  } finally {
    mcp.close();
    await browser.close();
  }

  if (!passed) {
    console.error(
      `[solve-text] FAIL: demo did not accept any prediction after ${MAX_ATTEMPTS} attempts. ` +
        `predictions=${JSON.stringify(predictions)}`,
    );
    process.exit(1);
  }

  log(`PASS: predictions tried=${JSON.stringify(predictions)} — demo confirmed correct.`);
  process.exit(0);
}

main().catch((err) => {
  console.error('[solve-text] ERROR:', err && err.stack ? err.stack : err);
  process.exit(1);
});
