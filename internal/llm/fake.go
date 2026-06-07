package llm

import "context"

// Fake is a deterministic Client for tests.
type Fake struct {
	VisionResp      string
	TranscribeResp  string
	VisionErr       error
	TranscribeErr   error
	VisionCalls     []VisionCall
	TranscribeCalls []TranscribeCall
}

// VisionCall records a Vision invocation.
type VisionCall struct {
	Prompt string
	Opts   Options
	Images []Image
}

// TranscribeCall records a Transcribe invocation.
type TranscribeCall struct {
	Filename string
	Opts     Options
	Audio    []byte
}

// Vision implements Client.
func (f *Fake) Vision(_ context.Context, prompt string, images []Image, opts Options) (string, error) {
	f.VisionCalls = append(f.VisionCalls, VisionCall{Prompt: prompt, Images: images, Opts: opts})
	return f.VisionResp, f.VisionErr
}

// Transcribe implements Client.
func (f *Fake) Transcribe(_ context.Context, audio []byte, filename string, opts Options) (string, error) {
	f.TranscribeCalls = append(f.TranscribeCalls, TranscribeCall{Audio: audio, Filename: filename, Opts: opts})
	return f.TranscribeResp, f.TranscribeErr
}
