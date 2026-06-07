package stealth

import (
	_ "embed"
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

//go:embed assets/evasions.js.tmpl
var evasionsTmplSrc string

// evasionTmpl is parsed once at startup; template.Must panics on bad syntax.
var evasionTmpl = template.Must(template.New("evasions").Parse(evasionsTmplSrc))

// ScriptParams selects identity and target engine for apply instructions.
// Fields ordered for fieldalignment (largest to smallest, then strings).
type ScriptParams struct {
	Seed    uint64 // deterministic generation; 0 → random
	OS      string // windows | macos | linux
	Browser string // chrome
	Locale  string // e.g. en-US, en-GB
	Engine  string // playwright | cdp | generic
}

// ApplyInfo tells the caller how to inject the script.
type ApplyInfo struct {
	How string `json:"how"`
	API string `json:"api"`
}

// ScriptResult is the injectable evasion bundle plus metadata.
type ScriptResult struct {
	Script           string      `json:"script"`
	Apply            ApplyInfo   `json:"apply"`
	Fingerprint      Fingerprint `json:"fingerprint"`
	IncludedEvasions []string    `json:"included_evasions"`
}

// evasionData is the template data bag.
type evasionData struct {
	WebGLVendor   string
	WebGLRenderer string
	Platform      string
	Languages     string // pre-rendered JS list, e.g. 'en-GB', 'en'
}

// languagesJS converts a locale string to a JS list of quoted language tags.
// "en-GB" → `'en-GB', 'en'`; "en" → `'en'`; "" → `'en-US', 'en'`
func languagesJS(locale string) string {
	if locale == "" {
		locale = "en-US"
	}
	parts := strings.SplitN(locale, "-", 2)
	if len(parts) == 2 && parts[0] != locale {
		return fmt.Sprintf("'%s', '%s'", locale, parts[0])
	}
	return fmt.Sprintf("'%s'", locale)
}

// StealthScript returns an evasion init script rendered from a coherent
// Fingerprint, plus engine-specific apply hints. The Fingerprint used for
// rendering is included in ScriptResult so callers can apply it without a
// second call. Empty params produce valid output (windows/chrome/en-US defaults).
func StealthScript(p ScriptParams) ScriptResult {
	fp := GenerateFingerprint(FingerprintParams{
		OS:      p.OS,
		Browser: p.Browser,
		Locale:  p.Locale,
		Seed:    p.Seed,
	})

	var buf bytes.Buffer
	// Execute errors are non-fatal — the template is static aside from the data
	// substitution; if it somehow fails, fall back to a descriptive comment.
	if err := evasionTmpl.Execute(&buf, evasionData{
		WebGLVendor:   fp.WebGLVendor,
		WebGLRenderer: fp.WebGLRenderer,
		Platform:      fp.Platform,
		Languages:     languagesJS(fp.Locale),
	}); err != nil {
		// Fallback: return an empty-IIFE comment so the script is valid JS.
		buf.Reset()
		fmt.Fprintf(&buf, "(() => { /* evasion render error: %v */ })();", err)
	}

	apply := ApplyInfo{How: "Inject before any page script runs.", API: "page.addInitScript(script)"}
	switch p.Engine {
	case "cdp":
		apply.API = "Page.addScriptToEvaluateOnNewDocument({source: script})"
	case "generic", "":
		apply.API = "evaluate the script on every new document before navigation"
	}

	return ScriptResult{
		Script:      buf.String(),
		Apply:       apply,
		Fingerprint: fp,
		IncludedEvasions: []string{
			"navigator.webdriver", "navigator.languages", "navigator.platform",
			"navigator.plugins", "chrome.runtime", "permissions.query",
			"WebGL vendor/renderer",
		},
	}
}
