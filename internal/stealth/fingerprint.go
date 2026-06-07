// Package stealth generates browser anti-bot artifacts (fingerprint profiles,
// evasion init scripts, humanized interaction plans, header sets) that the
// caller injects/applies. It never drives a browser.
package stealth

import (
	"fmt"
	"math/rand/v2"
)

// FingerprintParams selects an identity. Seed makes generation deterministic.
type FingerprintParams struct {
	OS      string // windows | macos | linux
	Browser string // chrome | firefox
	Locale  string // e.g. en-US
	Seed    uint64
}

// Fingerprint is a coherent, internally-consistent browser identity.
type Fingerprint struct {
	UserAgent           string `json:"user_agent"`
	SecCHUA             string `json:"sec_ch_ua"`
	SecCHUAPlatform     string `json:"sec_ch_ua_platform"`
	Platform            string `json:"platform"`
	Locale              string `json:"locale"`
	Timezone            string `json:"timezone"`
	Viewport            string `json:"viewport"`
	WebGLVendor         string `json:"webgl_vendor"`
	WebGLRenderer       string `json:"webgl_renderer"`
	HardwareConcurrency int    `json:"hardware_concurrency"`
	DeviceMemory        int    `json:"device_memory"`
}

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

// GenerateFingerprint returns a coherent identity. Same Seed → same output.
func GenerateFingerprint(p FingerprintParams) Fingerprint {
	if p.OS == "" {
		p.OS = "windows"
	}
	if p.Browser == "" {
		p.Browser = "chrome"
	}
	if p.Locale == "" {
		p.Locale = "en-US"
	}
	r := rand.New(rand.NewPCG(p.Seed, p.Seed^0x9e3779b97f4a7c15)) //nolint:gosec // math/rand intentional: fingerprint generation is not a security primitive

	chromeMajor := pick(r, []string{"131", "132", "133"})
	var ua, platform, chPlatform, webglVendor, webglRenderer, tz string
	switch p.OS {
	case "macos":
		platform, chPlatform = "MacIntel", `"macOS"`
		ua = fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (Apple)", "ANGLE (Apple, Apple M2, OpenGL 4.1)"
		tz = "America/Los_Angeles"
	case "linux":
		platform, chPlatform = "Linux x86_64", `"Linux"`
		ua = fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (Mesa)", "ANGLE (Mesa, llvmpipe, OpenGL 4.5)"
		tz = "Europe/London"
	default: // windows
		platform, chPlatform = "Win32", `"Windows"`
		ua = fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeMajor)
		webglVendor, webglRenderer = "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11, D3D11)"
		tz = "America/New_York"
	}
	secCHUA := fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not?A_Brand";v="24"`, chromeMajor, chromeMajor)
	viewport := pick(r, []string{"1920x1080", "1536x864", "1440x900", "1366x768"})
	return Fingerprint{
		UserAgent:           ua,
		SecCHUA:             secCHUA,
		SecCHUAPlatform:     chPlatform,
		Platform:            platform,
		Locale:              p.Locale,
		Timezone:            tz,
		Viewport:            viewport,
		WebGLVendor:         webglVendor,
		WebGLRenderer:       webglRenderer,
		HardwareConcurrency: pick(r, []int{4, 8, 12, 16}),
		DeviceMemory:        pick(r, []int{4, 8, 16}),
	}
}
