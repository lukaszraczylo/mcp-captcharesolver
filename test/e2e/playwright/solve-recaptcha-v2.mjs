// BEST-EFFORT browser e2e: attempt the LIVE 2captcha reCAPTCHA v2 demo.
//
//   server (brain) ── solve_grid_captcha / plan_interaction ──┐
//   Playwright (hands) ── checkbox/screenshot/clicks/verify ──> reads g-recaptcha-response
//
// Target: https://2captcha.com/demo/recaptcha-v2 (real Google reCAPTCHA v2). Google
// aggressively resists headless/automation, so obtaining a token is NOT guaranteed. This
// script is BEST-EFFORT: it logs the outcome and EXITS 0 even with no token. It exits
// non-zero ONLY on a real script/transport error (e.g. server crash, navigation failure).
//
// AUTHORIZED USE ONLY — this targets 2captcha's public reCAPTCHA demo page.
//
// Required env (passed straight through to the spawned server):
//   CAPTCHA_LLM_PROVIDER=openai-compatible
//   CAPTCHA_LLM_BASE_URL=https://llmgw.h.raczylo.com
//   CAPTCHA_LLM_MODEL=openai/gpt-5.4
//   CAPTCHA_LLM_TIMEOUT=120s        (optional)
//   CAPTCHA_LLM_API_KEY=...         (optional; gateway is keyless)
//   CAPTCHA_BIN=/abs/path           (optional; defaults to repo bin/captcha-solver-mcp)

import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { chromium } from 'playwright';
import { McpStdioClient } from '../../../examples/mcp-client.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, '../../..');

const TARGET_URL = 'https://2captcha.com/demo/recaptcha-v2';
const CAPTCHA_BIN = process.env.CAPTCHA_BIN || path.join(REPO_ROOT, 'bin', 'captcha-solver-mcp');
const HEADLESS = process.env.CAPTCHA_E2E_HEADLESS !== '0';
// Opt-in headed/slow-mo mode. Real Google reCAPTCHA rarely mints a token in
// headless Chromium; a headed window backed by a real browser profile improves
// the odds. Set CAPTCHA_E2E_HEADED=1 to launch visible with slow-mo.
const HEADED = process.env.CAPTCHA_E2E_HEADED === '1';
const MAX_ROUNDS = 5; // dynamic grids re-render; cap the loop.

const serverEnv = {
  CAPTCHA_LLM_PROVIDER: process.env.CAPTCHA_LLM_PROVIDER || 'openai-compatible',
  CAPTCHA_LLM_BASE_URL: process.env.CAPTCHA_LLM_BASE_URL,
  CAPTCHA_LLM_API_KEY: process.env.CAPTCHA_LLM_API_KEY,
  CAPTCHA_LLM_MODEL: process.env.CAPTCHA_LLM_MODEL,
  CAPTCHA_LLM_TIMEOUT: process.env.CAPTCHA_LLM_TIMEOUT || '120s',
};

function log(...args) {
  console.log('[recaptcha-v2]', ...args);
}

// Centroids come back from the Go imageutil.Point struct, which has no JSON tags,
// so it serializes with capitalized keys {X,Y}. Tolerate both shapes.
function centroidXY(c) {
  if (!c) return null;
  const x = c.x ?? c.X;
  const y = c.y ?? c.Y;
  if (typeof x !== 'number' || typeof y !== 'number') return null;
  return { x, y };
}

async function readToken(page) {
  return page.evaluate(() => {
    const el = document.querySelector('textarea[name="g-recaptcha-response"]');
    return el ? el.value : '';
  });
}

async function main() {
  if (HEADED) {
    log(
      'headed mode ON (CAPTCHA_E2E_HEADED=1): launching a visible, slow-mo browser. ' +
        'A headed/real-profile browser improves the odds of reCAPTCHA minting a token.',
    );
  } else {
    log(
      'headed mode OFF: running headless. Set CAPTCHA_E2E_HEADED=1 for a visible, ' +
        'real-profile attempt that improves the odds of reCAPTCHA minting a token.',
    );
  }

  const mcp = new McpStdioClient(CAPTCHA_BIN, serverEnv);
  await mcp.initialize();

  // Coherent identity + matching headers + evasion init script.
  const fp = await mcp.callTool('generate_fingerprint', {
    os: 'macos',
    browser: 'chrome',
    locale: 'en-US',
    seed: 42,
  });
  const stealth = await mcp.callTool('get_stealth_script', { engine: 'playwright' });
  log('stealth evasions:', stealth.included_evasions);

  const headers = (fp.header_set && fp.header_set.headers) || {};
  const userAgent = headers['User-Agent'] || headers['user-agent'];
  const [vpW, vpH] = (fp.fingerprint && fp.fingerprint.viewport
    ? fp.fingerprint.viewport
    : '1366x768'
  )
    .split('x')
    .map((n) => parseInt(n, 10));
  const locale = (fp.fingerprint && fp.fingerprint.locale) || 'en-US';

  const browser = await chromium.launch(
    HEADED ? { headless: false, slowMo: 80 } : { headless: HEADLESS },
  );
  const context = await browser.newContext({
    userAgent,
    locale,
    viewport: { width: vpW || 1366, height: vpH || 768 },
    extraHTTPHeaders: headers,
  });
  await context.addInitScript({ content: stealth.script });
  const page = await context.newPage();

  let token = '';

  try {
    await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded', timeout: 60000 });

    // Click the "I'm not a robot" checkbox inside the anchor iframe.
    const anchor = page.frameLocator('iframe[title="reCAPTCHA"]');
    await anchor.locator('#recaptcha-anchor').click({ timeout: 20000 });
    await page.waitForTimeout(1500);

    // If the checkbox alone passed (low-friction path), we're done.
    token = await readToken(page);
    if (token) {
      log('token obtained from checkbox alone (no image challenge).');
    } else {
      // Image challenge lives in the "challenge" iframe.
      const challenge = page.frameLocator('iframe[title*="challenge"]');

      for (let round = 1; round <= MAX_ROUNDS; round++) {
        let instruction = '';
        try {
          instruction = (
            await challenge.locator('.rc-imageselect-instructions').innerText({ timeout: 8000 })
          )
            .replace(/\s+/g, ' ')
            .trim();
        } catch {
          log(`round ${round}: no challenge instructions visible; stopping.`);
          break;
        }

        const gridTable = challenge.locator('table');
        const box = await gridTable.boundingBox();
        if (!box) {
          log(`round ${round}: could not locate the grid table; stopping.`);
          break;
        }

        const cells = await challenge.locator('table td').count();
        const cols = Math.max(1, Math.round(Math.sqrt(cells)));
        const rows = Math.max(1, Math.round(cells / cols));

        const png = await gridTable.screenshot();
        const screenshot = `data:image/png;base64,${png.toString('base64')}`;

        const grid = await mcp.callTool('solve_grid_captcha', {
          screenshot,
          instruction,
          rows,
          cols,
          image_width: Math.round(box.width),
          image_height: Math.round(box.height),
          captcha_type: 'recaptcha',
        });
        log(
          `round ${round}: instruction="${instruction}" tiles=${JSON.stringify(grid.tiles)} ` +
            `conf=${grid.confidence} recheck=${grid.recheck_recommended}`,
        );

        if (grid.tiles && grid.tiles.length > 0) {
          // Map tiles -> page-absolute click targets (prefer brain centroids).
          const clickActions = grid.tiles.map((tileIndex, i) => {
            const c = centroidXY(grid.centroids && grid.centroids[i]);
            const localX = c ? c.x : ((tileIndex % cols) + 0.5) * (box.width / cols);
            const localY = c ? c.y : (Math.floor(tileIndex / cols) + 0.5) * (box.height / rows);
            return { type: 'click', x: Math.round(box.x + localX), y: Math.round(box.y + localY) };
          });

          const plan = await mcp.callTool('plan_interaction', { actions: clickActions, seed: 7 });
          for (const step of plan.steps) {
            if (step.type === 'move') await page.mouse.move(step.x, step.y);
            else if (step.type === 'down') await page.mouse.down();
            else if (step.type === 'up') await page.mouse.up();
            if (step.at_ms != null) await page.waitForTimeout(8);
          }
        }

        if (grid.recheck_recommended && grid.tiles && grid.tiles.length > 0) {
          await page.waitForTimeout(900); // dynamic tiles fade in; re-screenshot next round.
          continue;
        }

        // Verify the current selection.
        try {
          await challenge.locator('#recaptcha-verify-button').click({ timeout: 8000 });
        } catch {
          log(`round ${round}: verify button not clickable; stopping.`);
          break;
        }
        await page.waitForTimeout(1800);

        token = await readToken(page);
        if (token) break;

        // No token yet — Google likely served a fresh challenge; loop and re-solve.
        log(`round ${round}: no token yet; re-checking for a new challenge.`);
      }
    }

    token = token || (await readToken(page));
  } catch (err) {
    // A genuine script/transport error -> non-zero exit.
    console.error('[recaptcha-v2] ERROR:', err && err.stack ? err.stack : err);
    mcp.close();
    await browser.close();
    process.exit(1);
  }

  mcp.close();
  await browser.close();

  if (token) {
    log(`OUTCOME: token obtained (length=${token.length}). reCAPTCHA v2 solved.`);
  } else {
    log(
      'OUTCOME: no token obtained. Real Google reCAPTCHA commonly resists headless automation; ' +
        'this is expected for the best-effort path. Exiting 0.',
    );
  }
  process.exit(0); // best-effort: success regardless of token, only errors fail.
}

main().catch((err) => {
  console.error('[recaptcha-v2] ERROR:', err && err.stack ? err.stack : err);
  process.exit(1);
});
