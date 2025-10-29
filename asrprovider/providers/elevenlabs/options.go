package elevenlabs

// Options contains ElevenLabs-specific fetch options.
type Options struct {
	// LanguageCode specifies the language code for transcription.
	// Common values: "zh" (Chinese), "en" (English), "auto" (auto-detection).
	// Default: "auto"
	LanguageCode string

	// TagAudioEvents indicates whether to tag audio events like music, applause, etc.
	// When enabled, the API will identify and tag non-speech audio events.
	// Default: false
	TagAudioEvents bool
}

// Validate validates the options and sets default values.
//
// Default values:
//   - LanguageCode: "auto" if not specified
//
// This method always returns nil as all option combinations are valid.
func (o *Options) Validate() error {
	// Set default language code
	if o.LanguageCode == "" {
		o.LanguageCode = "auto"
	}

	return nil
}
