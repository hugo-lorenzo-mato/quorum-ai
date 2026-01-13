package service

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimiterConfig configures a rate limiter.
type RateLimiterConfig struct {
	MaxTokens  float64 // Maximum bucket capacity
	RefillRate float64 // Tokens added per second
}

// DefaultRateLimiterConfig returns default configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1, // 1 token per second
	}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		tokens:     cfg.MaxTokens,
		maxTokens:  cfg.MaxTokens,
		refillRate: cfg.RefillRate,
		lastRefill: time.Now(),
	}
}

// Acquire blocks until a token is available or context is cancelled.
func (r *RateLimiter) Acquire(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.refill()

		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time for next token
		waitTime := time.Duration(float64(time.Second) / r.refillRate)
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again
		}
	}
}

// TryAcquire attempts to acquire a token without blocking.
func (r *RateLimiter) TryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// AcquireN blocks until n tokens are available.
func (r *RateLimiter) AcquireN(ctx context.Context, n int) error {
	for i := 0; i < n; i++ {
		if err := r.Acquire(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Available returns the current number of available tokens.
func (r *RateLimiter) Available() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	return r.tokens
}

// MaxTokens returns the maximum capacity.
func (r *RateLimiter) MaxTokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxTokens
}

// RefillRate returns the current refill rate.
func (r *RateLimiter) RefillRate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.refillRate
}

// refill adds tokens based on elapsed time.
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	r.lastRefill = now

	tokensToAdd := elapsed.Seconds() * r.refillRate
	r.tokens = minFloat(r.maxTokens, r.tokens+tokensToAdd)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// RateLimiterRegistry manages rate limiters for multiple adapters.
type RateLimiterRegistry struct {
	limiters map[string]*RateLimiter
	configs  map[string]RateLimiterConfig
	mu       sync.RWMutex
}

// NewRateLimiterRegistry creates a new registry.
func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{
		limiters: make(map[string]*RateLimiter),
		configs:  defaultAdapterConfigs(),
	}
}

// defaultAdapterConfigs returns default rate limit configs per adapter.
func defaultAdapterConfigs() map[string]RateLimiterConfig {
	return map[string]RateLimiterConfig{
		"claude": {
			MaxTokens:  5,
			RefillRate: 0.5, // 1 request per 2 seconds
		},
		"gemini": {
			MaxTokens:  10,
			RefillRate: 1, // 1 request per second
		},
		"codex": {
			MaxTokens:  3,
			RefillRate: 0.2, // 1 request per 5 seconds
		},
		"copilot": {
			MaxTokens:  5,
			RefillRate: 0.5,
		},
	}
}

// Get returns the rate limiter for an adapter.
func (r *RateLimiterRegistry) Get(adapter string) *RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, ok := r.limiters[adapter]; ok {
		return limiter
	}

	// Create new limiter
	cfg, ok := r.configs[adapter]
	if !ok {
		cfg = DefaultRateLimiterConfig()
	}

	limiter := NewRateLimiter(cfg)
	r.limiters[adapter] = limiter
	return limiter
}

// SetConfig updates the configuration for an adapter.
func (r *RateLimiterRegistry) SetConfig(adapter string, cfg RateLimiterConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configs[adapter] = cfg
	// Recreate limiter with new config
	r.limiters[adapter] = NewRateLimiter(cfg)
}

// Status returns rate limiter status for all adapters.
func (r *RateLimiterRegistry) Status() map[string]RateLimiterStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]RateLimiterStatus)
	for name, limiter := range r.limiters {
		status[name] = RateLimiterStatus{
			Available:  limiter.Available(),
			MaxTokens:  limiter.MaxTokens(),
			RefillRate: limiter.RefillRate(),
		}
	}
	return status
}

// List returns all adapter names with configured limiters.
func (r *RateLimiterRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.configs))
	for name := range r.configs {
		names = append(names, name)
	}
	return names
}

// RateLimiterStatus contains status information.
type RateLimiterStatus struct {
	Available  float64
	MaxTokens  float64
	RefillRate float64
}

// AdaptiveRateLimiter adjusts rate based on error feedback.
type AdaptiveRateLimiter struct {
	*RateLimiter
	adaptiveMu     sync.Mutex
	consecutiveOK  int
	consecutiveErr int
	minRefillRate  float64
	maxRefillRate  float64
}

// NewAdaptiveRateLimiter creates an adaptive rate limiter.
func NewAdaptiveRateLimiter(cfg RateLimiterConfig) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		RateLimiter:   NewRateLimiter(cfg),
		minRefillRate: cfg.RefillRate * 0.1,
		maxRefillRate: cfg.RefillRate * 2,
	}
}

// RecordSuccess indicates a successful request.
func (a *AdaptiveRateLimiter) RecordSuccess() {
	a.adaptiveMu.Lock()
	defer a.adaptiveMu.Unlock()

	a.consecutiveOK++
	a.consecutiveErr = 0

	// Increase rate after 5 consecutive successes
	if a.consecutiveOK >= 5 {
		a.RateLimiter.mu.Lock()
		newRate := a.RateLimiter.refillRate * 1.1
		if newRate <= a.maxRefillRate {
			a.RateLimiter.refillRate = newRate
		}
		a.RateLimiter.mu.Unlock()
		a.consecutiveOK = 0
	}
}

// RecordError indicates a rate limit error.
func (a *AdaptiveRateLimiter) RecordError() {
	a.adaptiveMu.Lock()
	defer a.adaptiveMu.Unlock()

	a.consecutiveErr++
	a.consecutiveOK = 0

	// Decrease rate immediately on error
	a.RateLimiter.mu.Lock()
	newRate := a.RateLimiter.refillRate * 0.5
	if newRate >= a.minRefillRate {
		a.RateLimiter.refillRate = newRate
	}
	a.RateLimiter.mu.Unlock()
}

// CurrentRefillRate returns the current refill rate (for testing).
func (a *AdaptiveRateLimiter) CurrentRefillRate() float64 {
	a.RateLimiter.mu.Lock()
	defer a.RateLimiter.mu.Unlock()
	return a.RateLimiter.refillRate
}
