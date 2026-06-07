// Reference: solve a reCAPTCHA v2 image-grid challenge with Playwright as the "hands"
// and captcha-solver-mcp as the "brain".
//
// The MCP server NEVER drives the browser and NEVER mints a token. Playwright does the
// navigation, screenshots, clicks, and token harvesting; the server only tells us which
// tiles match and produces stealth artifacts + a humanized click timeline.
//
// Run it yourself (this repo does not vendor node_modules):
//   npm i playwright
//   node examples/playwright/solve-recaptcha.mjs https://your-own-test-page.example/recaptcha
//
// Required env (point at your own vision-capable LLM endpoint):
//   CAPTCHA_BIN=/abs/path/to/bin/captcha-solver-mcp
//   CAPTCHA_LLM_PROVIDER=openai-compatible
//   CAPTCHA_LLM_BASE_URL=https://your-gateway.example.com/v1
//   CAPTCHA_LLM_API_KEY=sk-...
//   CAPTCHA_LLM_MODEL=openai/gpt-5.4
//
// AUTHORIZED USE ONLY. Run this against captcha integrations you own or are permitted to
// test. Do not use it to evade protections on sites you do not control.

import { chromium } from 'playwright';
import { McpStdioClient } from '../mcp-client.mjs';

const TARGET_URL = process.argv[2];
const CAPTCHA_BIN = process.env.CAPTCHA_BIN || '../../bin/captcha-solver-mcp';
const MAX_ROUNDS = 6; // dynamic grids re-render; cap the loop

if (!TARGET_URL) {
  console.error('usage: node solve-recaptcha.mjs <url-of-your-own-recaptcha-test-page>');
  process.exit(2);
}

// LLM config is passed straight through to the server process.
const serverEnv = {
  CAPTCHA_LLM_PROVIDER: process.env.CAPTCHA_LLM_PROVIDER,
  CAPTCHA_LLM_BASE_URL: process.env.CAPTCHA_LLM_BASE_URL,
  CAPTCHA_LLM_API_KEY: process.env.CAPTCHA_LLM_API_KEY,
  CAPTCHA_LLM_MODEL: process.env.CAPTCHA_LLM_MODEL,
};

const mcp = new McpStdioClient(CAPTCHA_BIN, serverEnv);
await mcp.initialize();

// 1. Prep identity: coherent fingerprint + matching headers, and the evasion init script.
//    A stable `seed` keeps the identity consistent across runs.
const fp = await mcp.callTool('generate_fingerprint', {
  os: 'macos',
  browser: 'chrome',
  locale: 'en-US',
  seed: 42,
});
const stealth = await mcp.callTool('get_stealth_script', { engine: 'playwright' });
console.log('stealth evasions:', stealth.included_evasions);

const browser = await chromium.launch({ headless: false });
const context = await browser.newContext({
  userAgent: fp.header_set['User-Agent'] || fp.header_set['user-agent'],
  locale: 'en-US',
  extraHTTPHeaders: fp.header_set, // apply the coherent header set
});
// Inject the evasion script BEFORE any page script runs.
await context.addInitScript({ content: stealth.script });

const page = await context.newPage();
await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' });

// 2. Trigger the challenge. Selectors here are illustrative — adapt to your test page.
//    Typically: click the "I'm not a robot" checkbox, then wait for the tile iframe.
await page.frameLocator('iframe[title="reCAPTCHA"]').locator('#recaptcha-anchor').click();

// The image challenge lives in a second iframe ("recaptcha challenge expires...").
const challenge = page.frameLocator('iframe[title*="challenge"]');

for (let round = 1; round <= MAX_ROUNDS; round++) {
  // Wait for the grid + instruction to be present.
  const instruction = (await challenge.locator('.rc-imageselect-instructions').innerText())
    .replace(/\s+/g, ' ')
    .trim();
  const gridTable = challenge.locator('table');
  const box = await gridTable.boundingBox();
  if (!box) throw new Error('could not locate the grid table');

  // Infer grid size from the rendered cells (3x3 or 4x4 are the common shapes).
  const cells = await challenge.locator('table td').count();
  const cols = Math.round(Math.sqrt(cells));
  const rows = Math.round(cells / cols);

  // 2b. Screenshot just the grid so pixel coordinates line up with the brain's centroids.
  const png = await gridTable.screenshot();
  const screenshot = `data:image/png;base64,${png.toString('base64')}`;

  // 3. Ask the brain which tiles to click. Pass image dims to get pixel centroids back.
  const grid = await mcp.callTool('solve_grid_captcha', {
    screenshot,
    instruction,
    rows,
    cols,
    image_width: Math.round(box.width),
    image_height: Math.round(box.height),
    captcha_type: 'recaptcha',
  });
  console.log(`round ${round}: instruction=${instruction} tiles=${grid.tiles} conf=${grid.confidence}`);

  if (!grid.tiles || grid.tiles.length === 0) break; // nothing matches -> done selecting

  // 4. Humanize: turn centroids (page-absolute) into a Bézier mouse/click timeline.
  const clickActions = grid.tiles.map((tileIndex, i) => {
    // Prefer brain-provided centroids; otherwise derive from the grid geometry.
    const c = grid.centroids && grid.centroids[i];
    const localX = c ? c.x : (tileIndex % cols + 0.5) * (box.width / cols);
    const localY = c ? c.y : (Math.floor(tileIndex / cols) + 0.5) * (box.height / rows);
    return { type: 'click', x: Math.round(box.x + localX), y: Math.round(box.y + localY) };
  });
  const plan = await mcp.callTool('plan_interaction', { actions: clickActions, seed: 7 });

  // Replay the humanized timeline against the real mouse.
  for (const step of plan.steps) {
    if (step.type === 'move') await page.mouse.move(step.x, step.y);
    else if (step.type === 'down') await page.mouse.down();
    else if (step.type === 'up') await page.mouse.up();
    // (typing steps are not used for grid clicks)
  }

  // 6. If the grid is dynamic, the brain told us to re-check: loop and re-screenshot.
  if (grid.recheck_recommended) {
    await page.waitForTimeout(800); // let new tiles fade in
    continue;
  }
  break;
}

// 5. Verify.
await challenge.locator('#recaptcha-verify-button').click();
await page.waitForTimeout(1500);

// 7. Harvest the token. The caller (us) reads it — the server never sees it.
const token = await page.evaluate(() => {
  const el = document.querySelector('textarea[name="g-recaptcha-response"]');
  return el ? el.value : '';
});
console.log('g-recaptcha-response token length:', token.length);
if (!token) console.warn('no token yet — challenge may still be unsolved');

mcp.close();
await browser.close();
