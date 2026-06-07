package captcha

import (
	"regexp"
	"strings"
)

var (
	reRecaptchaV3 = regexp.MustCompile(`recaptcha/api\.js\?render=([\w-]+)`)
	reSitekey     = regexp.MustCompile(`data-sitekey="([^"]+)"`)
	rePkey        = regexp.MustCompile(`data-pkey="([^"]+)"`)
)

// DetectHTML scans raw page HTML and returns deterministic detections for known
// captcha providers. v3/Turnstile are reported with LLMSolvable=false.
func DetectHTML(html string) []Detection {
	var out []Detection
	// reCAPTCHA v3 first (script render param). v3 has no visual challenge.
	if m := reRecaptchaV3.FindStringSubmatch(html); m != nil {
		out = append(out, Detection{
			Type: "recaptcha_v3", Sitekey: m[1], LLMSolvable: false,
			Notes: "score/behavioral — not LLM-solvable; needs browser reputation",
		})
	} else if strings.Contains(html, "g-recaptcha") || strings.Contains(html, "google.com/recaptcha/api2") {
		out = append(out, Detection{Type: "recaptcha_v2", Sitekey: firstSitekey(html), LLMSolvable: true,
			Notes: "image-grid; LLM returns tiles, caller clicks + harvests token"})
	}
	if strings.Contains(html, "h-captcha") || strings.Contains(html, "hcaptcha.com") {
		out = append(out, Detection{Type: "hcaptcha", Sitekey: firstSitekey(html), LLMSolvable: true,
			Notes: "image-grid; LLM returns tiles, caller clicks + harvests token"})
	}
	if strings.Contains(html, "cf-turnstile") || strings.Contains(html, "challenges.cloudflare.com") {
		out = append(out, Detection{Type: "turnstile", Sitekey: firstSitekey(html), LLMSolvable: false,
			Notes: "proof-of-work/behavioral — not LLM-solvable"})
	}
	if strings.Contains(html, "arkoselabs") || strings.Contains(html, "funcaptcha") {
		out = append(out, Detection{Type: "funcaptcha", Sitekey: firstPkey(html), LLMSolvable: true,
			Notes: "rotation/visual — best-effort"})
	}
	return out
}

func firstSitekey(html string) string {
	if m := reSitekey.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

func firstPkey(html string) string {
	if m := rePkey.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}
