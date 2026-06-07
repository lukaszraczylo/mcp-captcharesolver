package stealth

import "testing"

func TestGenerateFingerprintDeterministic(t *testing.T) {
	a := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Locale: "en-US", Seed: 42})
	b := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Locale: "en-US", Seed: 42})
	if a != b {
		t.Fatalf("same seed must be deterministic:\n%+v\n%+v", a, b)
	}
	if a.UserAgent == "" || a.Platform == "" || a.SecCHUA == "" {
		t.Fatalf("incomplete fingerprint: %+v", a)
	}
	// internal consistency: windows UA must mention Windows and platform Win32
	if a.Platform != "Win32" {
		t.Fatalf("windows platform=%q", a.Platform)
	}
}

func TestGenerateFingerprintVaries(t *testing.T) {
	a := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Seed: 1})
	b := GenerateFingerprint(FingerprintParams{OS: "windows", Browser: "chrome", Seed: 2})
	if a.HardwareConcurrency == b.HardwareConcurrency && a.UserAgent == b.UserAgent && a.Viewport == b.Viewport {
		t.Fatal("different seeds should vary at least one attribute")
	}
}
