package cli

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// AgentFactory creates an agent from configuration.
type AgentFactory func(cfg AgentConfig) (core.Agent, error)

// Registry manages available CLI agents.
type Registry struct {
	factories    map[string]AgentFactory
	agents       map[string]core.Agent
	configs      map[string]AgentConfig
	eventHandler core.AgentEventHandler // shared event handler for all agents
	mu           sync.RWMutex
}

// NewRegistry creates a new agent registry.
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]AgentFactory),
		agents:    make(map[string]core.Agent),
		configs:   make(map[string]AgentConfig),
	}

	// Register built-in factories
	r.registerBuiltins()

	return r
}

// registerBuiltins registers the default agent factories.
func (r *Registry) registerBuiltins() {
	r.RegisterFactory("claude", NewClaudeAdapter)
	r.RegisterFactory("gemini", NewGeminiAdapter)
	r.RegisterFactory("codex", NewCodexAdapter)
	r.RegisterFactory("copilot", NewCopilotAdapter)
}

// RegisterFactory registers a factory for an agent type.
func (r *Registry) RegisterFactory(name string, factory AgentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Register adds an agent directly to the registry.
func (r *Registry) Register(name string, agent core.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = agent
	return nil
}

// Configure sets configuration for an agent.
func (r *Registry) Configure(name string, cfg AgentConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cfg
	// Clear cached agent to force re-creation
	delete(r.agents, name)
}

// Get returns an agent by name, creating it if necessary.
func (r *Registry) Get(name string) (core.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Return cached agent if available
	if agent, ok := r.agents[name]; ok {
		return agent, nil
	}

	// Get factory
	factory, ok := r.factories[name]
	if !ok {
		return nil, core.ErrNotFound("agent", name)
	}

	// Get configuration
	cfg, ok := r.configs[name]
	if !ok {
		cfg = defaultConfig(name)
	}

	// Create agent
	agent, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating agent %s: %w", name, err)
	}

	// Configure event handler if one is set
	if r.eventHandler != nil {
		if sc, ok := agent.(core.StreamingCapable); ok {
			sc.SetEventHandler(r.eventHandler)
		}
	}

	// Cache agent
	r.agents[name] = agent
	return agent, nil
}

// List returns names of all registered agents.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// ListEnabled returns names of configured and enabled agents.
func (r *Registry) ListEnabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0)
	for name := range r.configs {
		names = append(names, name)
	}
	return names
}

// Has checks if an agent is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// GetCapabilities returns capabilities for an agent.
func (r *Registry) GetCapabilities(name string) (core.Capabilities, error) {
	agent, err := r.Get(name)
	if err != nil {
		return core.Capabilities{}, err
	}
	return agent.Capabilities(), nil
}

// Ping checks if an agent is available.
func (r *Registry) Ping(ctx context.Context, name string) error {
	agent, err := r.Get(name)
	if err != nil {
		return err
	}
	return agent.Ping(ctx)
}

// PingAll checks availability of all configured agents.
func (r *Registry) PingAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	names := make([]string, 0, len(r.configs))
	for name := range r.configs {
		names = append(names, name)
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.Ping(ctx, name)
			mu.Lock()
			results[name] = err
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

// Available returns agents that pass Ping.
func (r *Registry) Available(ctx context.Context) []string {
	results := r.PingAll(ctx)
	available := make([]string, 0)
	for name, err := range results {
		if err == nil {
			available = append(available, name)
		}
	}
	return available
}

// Clear removes all cached agents.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents = make(map[string]core.Agent)
}

// defaultConfig returns default configuration for an agent.
func defaultConfig(name string) AgentConfig {
	defaults := map[string]AgentConfig{
		"claude": {
			Name:        "claude",
			Path:        "claude",
			Model:       "claude-sonnet-4-20250514",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		},
		"gemini": {
			Name:        "gemini",
			Path:        "gemini",
			Model:       "gemini-2.5-flash",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		},
		"codex": {
			Name:        "codex",
			Path:        "codex",
			Model:       "gpt-5.1-codex",
			MaxTokens:   4096,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		},
		"copilot": {
			Name:        "copilot",
			Path:        "copilot",
			Model:       "claude-sonnet-4-5",
			MaxTokens:   16384,
			Temperature: 0.7,
			Timeout:     5 * time.Minute,
		},
	}

	if cfg, ok := defaults[name]; ok {
		return cfg
	}

	return AgentConfig{
		Name:        name,
		MaxTokens:   4096,
		Temperature: 0.7,
		Timeout:     5 * time.Minute,
	}
}

// Ensure Registry implements core.AgentRegistry
var _ core.AgentRegistry = (*Registry)(nil)

// LogCallbackSetter is implemented by agents that support real-time log streaming.
type LogCallbackSetter interface {
	SetLogCallback(cb LogCallback)
}

// SetLogCallback sets a log callback on all agents that support it.
// The callback receives stderr lines in real-time during execution.
func (r *Registry) SetLogCallback(cb LogCallback) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range r.agents {
		if setter, ok := agent.(LogCallbackSetter); ok {
			setter.SetLogCallback(cb)
		}
	}
}

// SetLogCallbackForAgent sets a log callback on a specific agent.
func (r *Registry) SetLogCallbackForAgent(name string, cb LogCallback) error {
	agent, err := r.Get(name)
	if err != nil {
		return err
	}
	if setter, ok := agent.(LogCallbackSetter); ok {
		setter.SetLogCallback(cb)
	}
	return nil
}

// SetEventHandler sets an event handler on all streaming-capable agents.
// The handler receives real-time streaming events during agent execution.
// New agents created after this call will also receive the handler.
func (r *Registry) SetEventHandler(handler core.AgentEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store handler for future agents
	r.eventHandler = handler

	// Apply to existing agents
	for _, agent := range r.agents {
		if sc, ok := agent.(core.StreamingCapable); ok {
			sc.SetEventHandler(handler)
		}
	}
}

// SetEventHandlerForAgent sets an event handler on a specific agent.
func (r *Registry) SetEventHandlerForAgent(name string, handler core.AgentEventHandler) error {
	agent, err := r.Get(name)
	if err != nil {
		return err
	}
	if sc, ok := agent.(core.StreamingCapable); ok {
		sc.SetEventHandler(handler)
	}
	return nil
}
