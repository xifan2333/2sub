package llm

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages all registered LLM providers.
//
// The registry is thread-safe and can be accessed concurrently.
// Providers are typically registered during package initialization using init() functions.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// globalRegistry is the default registry used by package-level functions.
var globalRegistry = &Registry{
	providers: make(map[string]Provider),
}

// Register registers a new LLM provider in the global registry.
//
// This function is typically called from a provider's init() function:
//
//	func init() {
//	    llm.Register(&MyProvider{})
//	}
//
// If a provider with the same name already exists, it will be replaced.
// This function is safe for concurrent use.
func Register(provider Provider) {
	globalRegistry.Register(provider)
}

// Get retrieves a provider by name from the global registry.
//
// Returns an error if the provider is not found.
// This function is safe for concurrent use.
//
// Example:
//
//	provider, err := llm.Get("openai")
//	if err != nil {
//	    // Provider not registered
//	}
func Get(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// List returns all registered provider names from the global registry.
//
// The order of names is not guaranteed.
// This function is safe for concurrent use.
//
// Example:
//
//	names := llm.List()
//	fmt.Printf("Available providers: %v\n", names)
func List() []string {
	return globalRegistry.List()
}

// Register registers a new provider to this registry.
//
// If a provider with the same name already exists, it will be replaced.
// This method is safe for concurrent use.
func (r *Registry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Get retrieves a provider by name from this registry.
//
// Returns an error if the provider is not found.
// This method is safe for concurrent use.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}
	return provider, nil
}

// List returns all registered provider names from this registry.
//
// The order of names is not guaranteed.
// This method is safe for concurrent use.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Chat is a convenience function that performs LLM chat completion.
//
// This is the recommended way to use the library for most use cases.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - providerName: Name of the provider to use (e.g., "openai", "claude", "gemini")
//   - opts: Provider options including API key, model, messages, etc.
//
// Returns the standardized chat result or an error.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	opts := &llm.Options{
//	    APIKey: "your-api-key",
//	    Model: "gpt-4",
//	    Messages: []llm.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	}
//	result, err := llm.Chat(ctx, "openai", opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Content)
func Chat(ctx context.Context, providerName string, opts *Options) (*StandardResult, error) {
	provider, err := Get(providerName)
	if err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	result, err := provider.Chat(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %w", err)
	}

	return result, nil
}
