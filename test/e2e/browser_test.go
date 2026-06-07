//go:build e2e && live

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// runBrowserScript shells out to a Playwright .mjs harness under test/e2e/playwright.
// It gates on CAPTCHA_E2E_LIVE=1, requires `node` and an installed Playwright Chromium,
// inherits the ambient CAPTCHA_LLM_* gateway config, and asserts a clean (exit 0) run.
func runBrowserScript(t *testing.T, script string) {
	t.Helper()

	if os.Getenv("CAPTCHA_E2E_LIVE") != "1" {
		t.Skip("CAPTCHA_E2E_LIVE != 1; skipping live browser e2e")
	}
	if os.Getenv("CAPTCHA_LLM_PROVIDER") == "" || os.Getenv("CAPTCHA_LLM_MODEL") == "" {
		t.Skip("CAPTCHA_LLM_PROVIDER / CAPTCHA_LLM_MODEL not set; skipping live browser e2e")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found on PATH; skipping live browser e2e")
	}
	if !chromiumInstalled() {
		t.Skip("Playwright Chromium not installed (run: npx playwright install chromium); skipping")
	}

	// cwd is test/e2e so the script path "playwright/<script>" resolves and pnpm's
	// node_modules above it is reachable for the `playwright` import.
	cmd := exec.Command("node", filepath.Join("playwright", script))
	cmd.Dir = "."
	cmd.Env = append(cmd.Environ(),
		"CAPTCHA_LLM_TIMEOUT="+envOr("CAPTCHA_LLM_TIMEOUT", "120s"),
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	// Generous ceiling: live network + a vision model + (recaptcha) several rounds.
	timer := time.AfterFunc(5*time.Minute, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	if err := cmd.Run(); err != nil {
		t.Fatalf("browser script %s failed: %v", script, err)
	}
}

// chromiumInstalled reports whether a Playwright Chromium build is present in the
// default browsers cache. Playwright also honors PLAYWRIGHT_BROWSERS_PATH.
func chromiumInstalled() bool {
	dirs := []string{}
	if p := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); p != "" {
		dirs = append(dirs, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, "Library", "Caches", "ms-playwright"), // macOS
			filepath.Join(home, ".cache", "ms-playwright"),             // linux
		)
	}
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, "chromium-*"))
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestBrowserSolveText drives the RELIABLE path: the live 2captcha text-captcha demo,
// solved via the MCP server + a real vision model, verified by the demo's own success
// indicator. Fails if the demo does not accept any prediction after retries.
func TestBrowserSolveText(t *testing.T) {
	runBrowserScript(t, "solve-text.mjs")
}

// TestBrowserRecaptchaV2 drives the BEST-EFFORT path: the live 2captcha reCAPTCHA v2
// demo. The script returns 0 whether or not a token is obtained (Google often resists
// headless), failing only on a real script/transport error.
func TestBrowserRecaptchaV2(t *testing.T) {
	runBrowserScript(t, "solve-recaptcha-v2.mjs")
}
