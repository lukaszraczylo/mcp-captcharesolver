package captcha

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `{"text":"ab12"}`, `{"text":"ab12"}`},
		{"prose wrapped", `Sure, here is the answer: {"text":"ab12"} hope it helps`, `{"text":"ab12"}`},
		{
			"markdown fenced",
			"```json\n{\"tiles\":[1,2,3]}\n```",
			`{"tiles":[1,2,3]}`,
		},
		{
			"nested object",
			`prefix {"a":{"b":1},"c":2} suffix`,
			`{"a":{"b":1},"c":2}`,
		},
		{
			"first balanced object wins",
			`For example {"text":"EXAMPLE"} but the real one is {"text":"REAL"}`,
			`{"text":"EXAMPLE"}`,
		},
		{"no brace", `no json here`, `no json here`},
		{"unbalanced returns from start", `tail {"text":"x"`, `{"text":"x"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractJSON(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
