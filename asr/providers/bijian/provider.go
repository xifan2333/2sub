// Package bijian provides an ASR provider implementation for Bijian (必剪),
// Bilibili's video editing application.
//
// Features:
//   - Word-level timestamps with character granularity
//   - Sentence-level segmentation
//   - Optional cookie authentication
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/asr"
//	    "github.com/xifan2333/2sub/asr/providers/bijian"
//	    _ "github.com/xifan2333/2sub/asr/providers/bijian"
//	)
//
//	opts := &bijian.Options{
//	    Cookie: "",  // Optional
//	}
//	result, err := asr.Transcribe(ctx, "bijian", "audio.mp3", opts)
package bijian

import (
	"context"

	"github.com/xifan2333/2sub/asr"
)

// Provider implements the ASR provider interface for Bijian (必剪).
//
// Bijian is Bilibili's video editing application that provides
// ASR services, primarily for Chinese language content.
type Provider struct{}

// Ensure Provider implements asr.Provider interface at compile time.
var _ asr.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	// This allows the provider to be used via asr.Get("bijian")
	// or asr.Transcribe(ctx, "bijian", ...).
	asr.Register(&Provider{})
}

// Name returns the provider's unique identifier.
//
// Returns "bijian".
func (p *Provider) Name() string {
	return "bijian"
}

// Fetch performs ASR transcription using Bijian API.
//
// The method executes a multi-step process:
//  1. Request upload authorization
//  2. Upload audio file in parts
//  3. Commit upload
//  4. Create transcription task
//  5. Poll for results
//
// Parameters:
//   - ctx: Context for cancellation and timeout (recommended: 5-10 minutes)
//   - audioPath: Path to the audio file (supports common formats)
//   - opts: Bijian-specific options (nil will use defaults)
//
// Returns the raw API response as map[string]interface{}.
func (p *Provider) Fetch(ctx context.Context, audioPath string, opts asr.FetchOptions) (asr.RawResult, error) {
	// Validate and convert options
	bijianOpts, ok := opts.(*Options)
	if !ok || bijianOpts == nil {
		bijianOpts = &Options{} // Use default options
	}

	if err := bijianOpts.Validate(); err != nil {
		return nil, err
	}

	// Perform the fetch operation
	return fetch(ctx, audioPath, bijianOpts)
}

// Parse converts the raw Bijian response to standardized format.
//
// The parser extracts:
//   - Complete transcription text
//   - Word-level timestamps (character granularity)
//   - Sentence-level segments
//
// All timestamps are converted to milliseconds.
//
// Returns an error if the response format is invalid or required fields are missing.
func (p *Provider) Parse(raw asr.RawResult) (*asr.StandardResult, error) {
	response, ok := raw.(map[string]interface{})
	if !ok {
		return nil, &ParseError{Message: "invalid raw result type, expected map[string]interface{}"}
	}

	return parse(response)
}
