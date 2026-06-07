package llm

import "context"

// Fake is a deterministic Client for tests.
type Fake struct {
	VisionResp      string
	TranscribeResp  string
	VisionErr       error
	TranscribeErr   error
	VisionResponses []string
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

// Vision implements Client. When VisionResponses is non-empty it returns the
// element matching the call index (clamped to the last for calls beyond the
// slice length); otherwise it falls back to VisionResp.
func (f *Fake) Vision(_ context.Context, prompt string, images []Image, opts Options) (string, error) {
	i := len(f.VisionCalls)
	f.VisionCalls = append(f.VisionCalls, VisionCall{Prompt: prompt, Images: images, Opts: opts})
	if f.VisionErr != nil {
		return "", f.VisionErr
	}
	if len(f.VisionResponses) > 0 {
		if i >= len(f.VisionResponses) {
			i = len(f.VisionResponses) - 1
		}
		return f.VisionResponses[i], nil
	}
	return f.VisionResp, f.VisionErr
}

// Transcribe implements Client.
func (f *Fake) Transcribe(_ context.Context, audio []byte, filename string, opts Options) (string, error) {
	f.TranscribeCalls = append(f.TranscribeCalls, TranscribeCall{Audio: audio, Filename: filename, Opts: opts})
	return f.TranscribeResp, f.TranscribeErr
}
