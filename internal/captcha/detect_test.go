package captcha

import "testing"

func TestDetectHTML(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		wantType     string
		wantSolvable bool
	}{
		{"recaptcha v2", `<div class="g-recaptcha" data-sitekey="6Lc_abc"></div>`, "recaptcha_v2", true},
		{"recaptcha v3", `<script src="https://www.google.com/recaptcha/api.js?render=6Lc_v3key"></script>`, "recaptcha_v3", false},
		{"hcaptcha", `<div class="h-captcha" data-sitekey="hk_123"></div>`, "hcaptcha", true},
		{"turnstile", `<div class="cf-turnstile" data-sitekey="0x4AAA"></div>`, "turnstile", false},
		{"funcaptcha", `<div class="arkoselabs" data-pkey="ABC"></div>`, "funcaptcha", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectHTML(tc.html)
			if len(got) == 0 {
				t.Fatalf("no detection")
			}
			if got[0].Type != tc.wantType || got[0].LLMSolvable != tc.wantSolvable {
				t.Fatalf("got %+v", got[0])
			}
		})
	}
}

// A v3 page often also carries a hidden g-recaptcha div; only v3 must fire.
func TestDetectHTMLV3SuppressesV2(t *testing.T) {
	html := `<script src="https://www.google.com/recaptcha/api.js?render=6Lc_v3key"></script>` +
		`<div class="g-recaptcha" data-sitekey="6Lc_hidden" style="display:none"></div>`
	got := DetectHTML(html)
	if len(got) != 1 {
		t.Fatalf("want exactly 1 detection, got %d: %+v", len(got), got)
	}
	if got[0].Type != "recaptcha_v3" {
		t.Fatalf("want recaptcha_v3, got %q", got[0].Type)
	}
	if got[0].LLMSolvable {
		t.Fatalf("recaptcha_v3 must not be LLMSolvable")
	}
}
