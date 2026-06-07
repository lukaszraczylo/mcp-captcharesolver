package stealth

import _ "embed"

//go:embed assets/evasions.js
var evasionsJS string

// ScriptParams selects the target engine for apply instructions.
type ScriptParams struct {
	Engine string // playwright | cdp | generic
}

// ApplyInfo tells the caller how to inject the script.
type ApplyInfo struct {
	How string `json:"how"`
	API string `json:"api"`
}

// ScriptResult is the injectable evasion bundle plus metadata.
type ScriptResult struct {
	Script           string    `json:"script"`
	Apply            ApplyInfo `json:"apply"`
	IncludedEvasions []string  `json:"included_evasions"`
}

// StealthScript returns the evasion init script and engine-specific apply hints.
func StealthScript(p ScriptParams) ScriptResult {
	apply := ApplyInfo{How: "Inject before any page script runs.", API: "page.addInitScript(script)"}
	switch p.Engine {
	case "cdp":
		apply.API = "Page.addScriptToEvaluateOnNewDocument({source: script})"
	case "generic", "":
		apply.API = "evaluate the script on every new document before navigation"
	}
	return ScriptResult{
		Script: evasionsJS,
		Apply:  apply,
		IncludedEvasions: []string{
			"navigator.webdriver", "navigator.languages", "navigator.plugins",
			"chrome.runtime", "permissions.query", "WebGL vendor/renderer",
		},
	}
}
