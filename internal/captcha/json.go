package captcha

import (
	"regexp"
	"strings"
)

// thinkRe matches reasoning-model scratchpad blocks (<think>…</think> or
// <thinking>…</thinking>). These can themselves contain JSON-looking objects,
// so they are stripped before the brace scan to avoid grabbing the wrong object.
var thinkRe = regexp.MustCompile(`(?s)<think(?:ing)?>.*?</think(?:ing)?>`)

// extractJSON returns the first balanced {...} object found in s, or s unchanged
// if no opening brace is present. It scans by brace depth so prose, markdown
// fences, or an example object before the real one do not corrupt parsing.
// Reasoning-model <think>…</think> blocks are stripped first so a JSON-looking
// object inside the scratchpad is not mistaken for the answer.
// Note: it does not account for braces inside string literals; that is
// acceptable for the short, structured captcha responses this handles.
func extractJSON(s string) string {
	s = thinkRe.ReplaceAllString(s, "")
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
