// Package asrprovider provides a unified interface for multiple ASR (Automatic Speech Recognition) providers.
//
// This package allows you to use different ASR services through a single, consistent API.
// All providers return results in a standardized format, making it easy to switch between
// providers or use multiple providers in the same application.
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/asrprovider"
//	    _ "github.com/xifan2333/2sub/asrprovider/providers/jianying"
//	)
//
//	func main() {
//	    ctx := context.Background()
//	    result, err := asrprovider.Transcribe(ctx, "jianying", "audio.mp3", nil)
//	    if err != nil {
//	        panic(err)
//	    }
//	    fmt.Println(result.Text)
//	}
package asr

import "context"

// Provider defines the interface that all ASR providers must implement.
//
// A provider is responsible for:
//   - Fetching raw transcription results from an ASR service
//   - Parsing those results into the standardized format
//
// Providers must be registered using the Register function, typically in their init() function.
type Provider interface {
	// Name returns the provider's unique identifier.
	// This name is used when calling Get() or Transcribe().
	//
	// Examples: "jianying", "elevenlabs", "bijian"
	Name() string

	// Fetch performs ASR transcription and returns the raw response.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - audioPath: Path to the audio file
	//   - opts: Provider-specific options (can be nil for defaults)
	//
	// The raw result type depends on the provider's API response format.
	// It is typically a map[string]interface{} for JSON responses.
	Fetch(ctx context.Context, audioPath string, opts FetchOptions) (RawResult, error)

	// Parse converts the raw response to the standardized format.
	//
	// This method should extract:
	//   - Complete transcription text
	//   - Word-level timestamps
	//   - Sentence segments (if available)
	//   - Language information (if available)
	//   - Speaker IDs (if available)
	//
	// All timestamps must be converted to milliseconds.
	Parse(raw RawResult) (*StandardResult, error)
}

// FetchOptions is a unified interface for provider-specific fetch options.
//
// Each provider defines its own options type that implements this interface.
// The Validate method should check the options and set default values.
//
// Example:
//
//	type MyOptions struct {
//	    APIKey string
//	    Language string
//	}
//
//	func (o *MyOptions) Validate() error {
//	    if o.Language == "" {
//	        o.Language = "auto"
//	    }
//	    return nil
//	}
type FetchOptions interface {
	// Validate validates the options and sets default values.
	// This method is called before Fetch() and should return an error
	// if the options are invalid.
	Validate() error
}

// RawResult represents the raw response from an ASR provider.
//
// The actual type depends on the provider's API response format.
// Most providers use map[string]interface{} for JSON responses.
type RawResult interface{}

// StandardResult represents the unified ASR result format.
//
// All providers must convert their responses to this standardized format.
// This allows applications to work with different providers using the same code.
//
// Note: Not all fields are populated by all providers. Check the provider's
// documentation to see which fields are supported.
type StandardResult struct {
	// Text is the complete transcription text.
	// This field is always populated.
	Text string `json:"text"`

	// Words contains word-level timestamps.
	// This field is always populated (may be empty if transcription failed).
	Words []Word `json:"words"`

	// Sentences contains sentence-level segments.
	// This field is optional and only populated by providers that support
	// sentence segmentation (e.g., JianYing, Bijian).
	Sentences []Sentence `json:"sentences,omitempty"`

	// Language is the detected or specified language code.
	// This field is optional and the format may vary by provider
	// (e.g., "zh-CN", "zho", "en").
	Language string `json:"language,omitempty"`
}

// Word represents word-level timestamp information.
//
// All timestamps are in milliseconds since the start of the audio.
type Word struct {
	// Text is the word content.
	Text string `json:"text"`

	// Start is the start time in milliseconds.
	Start int64 `json:"start"`

	// End is the end time in milliseconds.
	End int64 `json:"end"`

	// SpeakerID identifies the speaker (optional).
	// This field is only populated by providers that support speaker diarization
	// (e.g., ElevenLabs).
	SpeakerID string `json:"speaker_id,omitempty"`
}

// Sentence represents sentence-level segment information.
//
// All timestamps are in milliseconds since the start of the audio.
type Sentence struct {
	// Text is the sentence content.
	Text string `json:"text"`

	// Start is the start time in milliseconds.
	Start int64 `json:"start"`

	// End is the end time in milliseconds.
	End int64 `json:"end"`

	// SpeakerID identifies the speaker (optional).
	// This field is only populated by providers that support speaker diarization
	// at the sentence level.
	SpeakerID string `json:"speaker_id,omitempty"`
}
