package stealth

import "testing"

func TestHeadersFor(t *testing.T) {
	fp := GenerateFingerprint(FingerprintParams{OS: "macos", Browser: "chrome", Locale: "en-GB", Seed: 5})
	h := HeadersFor(fp)
	if h.Headers["User-Agent"] != fp.UserAgent {
		t.Fatalf("UA mismatch")
	}
	if h.Headers["Accept-Language"] == "" || h.Headers["Sec-CH-UA-Platform"] == "" {
		t.Fatalf("missing client hints: %+v", h.Headers)
	}
	if len(h.Advisories) == 0 {
		t.Fatal("expected TLS/JA3 advisory")
	}
}
