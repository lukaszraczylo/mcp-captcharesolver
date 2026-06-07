package stealth

import (
	"strings"
	"testing"
)

func TestStealthScript(t *testing.T) {
	res := StealthScript(ScriptParams{Engine: "playwright"})
	if res.Script == "" {
		t.Fatal("empty script")
	}
	for _, want := range []string{"navigator.webdriver", "languages", "WebGL", "chrome"} {
		if !strings.Contains(res.Script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
	if res.Apply.How == "" || len(res.IncludedEvasions) == 0 {
		t.Fatalf("missing apply metadata: %+v", res)
	}
}

func TestStealthScriptCoherence(t *testing.T) {
	p := ScriptParams{OS: "macos", Locale: "en-GB", Seed: 7}
	res := StealthScript(p)

	// Independently generate the same fingerprint.
	fp := GenerateFingerprint(FingerprintParams{OS: "macos", Locale: "en-GB", Seed: 7})

	// Script must contain the fingerprint's WebGL and platform values.
	for _, want := range []string{fp.WebGLVendor, fp.WebGLRenderer, fp.Platform} {
		if !strings.Contains(res.Script, want) {
			t.Errorf("script missing %q", want)
		}
	}
	// Script must contain navigator.platform spoof.
	if !strings.Contains(res.Script, "navigator.platform") {
		t.Error("script missing navigator.platform spoof")
	}
	// ScriptResult.Fingerprint must equal independently generated fingerprint.
	if res.Fingerprint != fp {
		t.Errorf("fingerprint mismatch:\n got  %+v\n want %+v", res.Fingerprint, fp)
	}
	// Template must not produce empty script.
	if res.Script == "" {
		t.Fatal("empty rendered script")
	}
}
