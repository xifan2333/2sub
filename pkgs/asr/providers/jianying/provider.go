// Package jianying provides an ASR provider implementation for JianYing (剪映),
// ByteDance's video editing application.
//
// Features:
//   - Word-level timestamps with phrase granularity
//   - Sentence-level segmentation
//   - Language detection (primarily Chinese)
//   - No authentication required
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/asr"
//	    "github.com/xifan2333/2sub/asr/providers/jianying"
//	    _ "github.com/xifan2333/2sub/asr/providers/jianying"
//	)
//
//	opts := &jianying.Options{
//	    StartTime: 0,
//	    EndTime:   6000,
//	}
//	result, err := asr.Transcribe(ctx, "jianying", "audio.mp3", opts)
package jianying

import (
	"context"

	"github.com/xifan2333/2sub/pkgs/asr"
)

// Provider implements the ASR provider interface for JianYing (剪映).
//
// JianYing is ByteDance's video editing application that provides
// ASR services with good support for Chinese language.
type Provider struct{}

// Ensure Provider implements asr.Provider interface at compile time.
var _ asr.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	// This allows the provider to be used via asr.Get("jianying")
	// or asr.Transcribe(ctx, "jianying", ...).
	asr.Register(&Provider{})
}

// Name returns the provider's unique identifier.
//
// Returns "jianying".
func (p *Provider) Name() string {
	return "jianying"
}

// Fetch performs ASR transcription using JianYing API.
//
// The method executes a multi-step process:
//  1. Obtain upload signature (AWS credentials)
//  2. Get upload authorization
//  3. Upload audio file
//  4. Verify upload
//  5. Commit upload
//  6. Submit transcription task
//  7. Query and wait for results
//
// Parameters:
//   - ctx: Context for cancellation and timeout (recommended: 5-10 minutes)
//   - audioPath: Path to the audio file (supports common formats like MP3, WAV)
//   - opts: JianYing-specific options (nil will use defaults)
//
// Returns the raw API response as map[string]interface{}.
func (p *Provider) Fetch(ctx context.Context, audioPath string, opts asr.FetchOptions) (asr.RawResult, error) {
	// Validate and convert options
	jianyingOpts, ok := opts.(*Options)
	if !ok || jianyingOpts == nil {
		jianyingOpts = &Options{} // Use default options
	}

	if err := jianyingOpts.Validate(); err != nil {
		return nil, err
	}

	// Perform the fetch operation
	return fetch(ctx, audioPath, jianyingOpts)
}

// Parse converts the raw JianYing response to standardized format.
//
// The parser extracts:
//   - Complete transcription text
//   - Word-level timestamps (phrase granularity)
//   - Sentence-level segments
//   - Language information (zh-CN)
//   - Speaker IDs (if available in response)
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
