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
