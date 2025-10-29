// Package elevenlabs provides an ASR provider implementation for ElevenLabs,
// an advanced speech-to-text API with speaker diarization support.
//
// Features:
//   - Word-level timestamps with character granularity
//   - Speaker diarization (identifies different speakers)
//   - Punctuation in transcription text
//   - Multi-language support
//   - Audio event tagging (optional)
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/asrprovider"
//	    "github.com/xifan2333/2sub/asrprovider/providers/elevenlabs"
//	    _ "github.com/xifan2333/2sub/asrprovider/providers/elevenlabs"
//	)
//
//	opts := &elevenlabs.Options{
//	    LanguageCode:   "zh",
//	    TagAudioEvents: false,
//	}
//	result, err := asrprovider.Transcribe(ctx, "elevenlabs", "audio.mp3", opts)
package elevenlabs

import (
	"context"

	"github.com/xifan2333/2sub/asrprovider"
)

// Provider implements the ASR provider interface for ElevenLabs.
//
// ElevenLabs provides advanced ASR capabilities including speaker diarization
// and support for multiple languages.
type Provider struct{}

// Ensure Provider implements asrprovider.Provider interface at compile time.
var _ asrprovider.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	// This allows the provider to be used via asrprovider.Get("elevenlabs")
	// or asrprovider.Transcribe(ctx, "elevenlabs", ...).
	asrprovider.Register(&Provider{})
}

// Name returns the provider's unique identifier.
//
// Returns "elevenlabs".
func (p *Provider) Name() string {
	return "elevenlabs"
}

// Fetch performs ASR transcription using ElevenLabs API.
//
// The method uploads the audio file via multipart form and receives
// the transcription result directly in the response.
//
// Parameters:
//   - ctx: Context for cancellation and timeout (recommended: 5-10 minutes)
//   - audioPath: Path to the audio file (supports common formats)
//   - opts: ElevenLabs-specific options (nil will use defaults)
//
// Returns the raw API response as map[string]interface{}.
func (p *Provider) Fetch(ctx context.Context, audioPath string, opts asrprovider.FetchOptions) (asrprovider.RawResult, error) {
	// Validate and convert options
	elevenlabsOpts, ok := opts.(*Options)
	if !ok || elevenlabsOpts == nil {
		elevenlabsOpts = &Options{} // Use default options
	}

	if err := elevenlabsOpts.Validate(); err != nil {
		return nil, err
	}

	// Perform the fetch operation
	return fetch(ctx, audioPath, elevenlabsOpts)
}

// Parse converts the raw ElevenLabs response to standardized format.
//
// The parser extracts:
//   - Complete transcription text with punctuation
//   - Word-level timestamps (character granularity)
//   - Speaker IDs for each word
//   - Language information
//
// Note: ElevenLabs does not provide sentence-level segmentation.
// All timestamps are converted to milliseconds.
//
// Returns an error if the response format is invalid or required fields are missing.
func (p *Provider) Parse(raw asrprovider.RawResult) (*asrprovider.StandardResult, error) {
	response, ok := raw.(map[string]interface{})
	if !ok {
		return nil, &ParseError{Message: "invalid raw result type, expected map[string]interface{}"}
	}

	return parse(response)
}
