package jianying

// Options contains JianYing-specific fetch options.
type Options struct {
	// StartTime is the audio start time in seconds (default: 0).
	// This allows transcribing only a portion of the audio file.
	StartTime float64

	// EndTime is the audio end time in seconds (default: 6000).
	// This allows transcribing only a portion of the audio file.
	// The default value of 6000 seconds (100 minutes) is sufficient for most use cases.
	EndTime float64
}

// Validate validates the options and sets default values.
//
// Default values:
//   - EndTime: 6000 seconds if not specified or zero
//
// Returns an error if:
//   - StartTime is negative
//   - EndTime is negative
//   - StartTime is greater than or equal to EndTime
func (o *Options) Validate() error {
	if o.EndTime == 0 {
		o.EndTime = 6000 // Set default end time
	}

	if o.StartTime < 0 {
		return &ValidationError{Field: "StartTime", Message: "must be non-negative"}
	}

	if o.EndTime < 0 {
		return &ValidationError{Field: "EndTime", Message: "must be non-negative"}
	}

	if o.StartTime >= o.EndTime {
		return &ValidationError{Field: "StartTime/EndTime", Message: "StartTime must be less than EndTime"}
	}

	return nil
}
