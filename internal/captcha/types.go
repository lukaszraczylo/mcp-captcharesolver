// Package captcha provides deterministic detection and LLM-backed solving of
// captcha challenges. It never drives a browser and never mints tokens.
package captcha

// Detection describes one captcha found on a page. Fields are ordered
// largest-alignment-first for fieldalignment.
type Detection struct {
	Type        string `json:"type"`
	Sitekey     string `json:"sitekey,omitempty"`
	IframeURL   string `json:"iframe_url,omitempty"`
	Notes       string `json:"notes,omitempty"`
	LLMSolvable bool   `json:"llm_solvable"`
}
