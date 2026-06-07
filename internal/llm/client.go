// Package llm defines a provider-agnostic LLM client for vision and audio
// transcription, with concrete HTTP implementations per provider.
package llm

import "context"

// Image is a decoded image to send to a vision model.
type Image struct {
	MediaType string // e.g. "image/png", "image/jpeg"
	Data      []byte
}

// Options are per-call overrides. Zero values mean "use the client default".
type Options struct {
	Model       string
	Language    string
	Temperature float64
	MaxTokens   int
}

// Client is the abstraction every provider implements and every handler depends on.
type Client interface {
	// Vision sends a prompt plus images and returns the model's text answer.
	Vision(ctx context.Context, prompt string, images []Image, opts Options) (string, error)
	// Transcribe converts audio bytes to text. filename carries the extension
	// for providers that infer the audio format from it. Returns ErrUnsupported
	// if the provider has no audio capability.
	Transcribe(ctx context.Context, audio []byte, filename string, opts Options) (string, error)
}

// ErrUnsupported is returned by Transcribe on providers without audio support.
type ErrUnsupported struct{ Provider string }

func (e ErrUnsupported) Error() string {
	return "audio transcription unsupported by provider " + e.Provider
}
