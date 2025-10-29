package bijian

// Options contains Bijian-specific fetch options.
type Options struct {
	// Cookie is the optional authentication cookie.
	// If not provided, the request may work without authentication
	// depending on the API's current access policy.
	Cookie string
}

// Validate validates the options and sets default values.
//
// This method always returns nil as Cookie is optional and there are
// no other validation requirements.
func (o *Options) Validate() error {
	// No specific validation needed for Bijian options
	// Cookie is optional
	return nil
}
