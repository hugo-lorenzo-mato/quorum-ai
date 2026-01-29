package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateLimiter_Acquire(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  3,
		RefillRate: 10, // Fast refill for testing
	}
	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Should acquire immediately (bucket starts full)
	start := time.Now()
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Error("first acquire should be immediate")
	}

	// Drain the bucket
	limiter.TryAcquire()
	limiter.TryAcquire()

	// Next acquire should wait for refill
	start = time.Now()
	err = limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	// With 10 tokens/second, should wait ~100ms
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("acquire should wait for refill, elapsed = %v", elapsed)
	}
}

func TestRateLimiter_TryAcquire(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  2,
		RefillRate: 0.1, // Very slow refill
	}
	limiter := NewRateLimiter(cfg)

	// Should acquire twice (bucket capacity = 2)
	if !limiter.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}
	if !limiter.TryAcquire() {
		t.Error("second TryAcquire should succeed")
	}

	// Third should fail (bucket empty)
	if limiter.TryAcquire() {
		t.Error("third TryAcquire should fail")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  5,
		RefillRate: 10, // 10 tokens per second
	}
	limiter := NewRateLimiter(cfg)

	// Drain bucket
	for limiter.TryAcquire() {
	}

	initial := limiter.Available()
	if initial > 0.5 {
		t.Errorf("Available after drain = %v, want ~0", initial)
	}

	// Wait for refill
	time.Sleep(200 * time.Millisecond)

	// Should have ~2 tokens (200ms * 10/s)
	available := limiter.Available()
	if available < 1.5 || available > 2.5 {
		t.Errorf("Available after 200ms = %v, want ~2", available)
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  1,
		RefillRate: 0.01, // Very slow
	}
	limiter := NewRateLimiter(cfg)

	// Drain bucket
	limiter.TryAcquire()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.Acquire(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Acquire() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRateLimiter_AcquireN(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  5,
		RefillRate: 100, // Fast
	}
	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	err := limiter.AcquireN(ctx, 3)
	if err != nil {
		t.Fatalf("AcquireN() error = %v", err)
	}

	// Should have ~2 tokens left
	available := limiter.Available()
	if available < 1.5 || available > 2.5 {
		t.Errorf("Available = %v, want ~2", available)
	}
}

func TestRateLimiterRegistry_Get(t *testing.T) {
	registry := NewRateLimiterRegistry()

	// Get limiter for known adapter
	claudeLimiter := registry.Get("claude")
	if claudeLimiter == nil {
		t.Fatal("Get(claude) should not return nil")
	}
	if claudeLimiter.MaxTokens() != 5 {
		t.Errorf("claude MaxTokens = %v, want 5", claudeLimiter.MaxTokens())
	}

	// Get limiter for unknown adapter (should use defaults)
	unknownLimiter := registry.Get("unknown")
	if unknownLimiter == nil {
		t.Fatal("Get(unknown) should not return nil")
	}
	if unknownLimiter.MaxTokens() != 10 {
		t.Errorf("unknown MaxTokens = %v, want 10 (default)", unknownLimiter.MaxTokens())
	}

	// Getting same adapter returns same limiter
	claudeLimiter2 := registry.Get("claude")
	if claudeLimiter != claudeLimiter2 {
		t.Error("Get should return same limiter for same adapter")
	}
}

func TestGetGlobalRateLimiter_Singleton(t *testing.T) {
	r1 := GetGlobalRateLimiter()
	r2 := GetGlobalRateLimiter()

	if r1 != r2 {
		t.Fatal("GetGlobalRateLimiter should return same instance")
	}
}

func TestRateLimiterRegistry_SetConfig(t *testing.T) {
	registry := NewRateLimiterRegistry()

	// Get initial limiter
	limiter1 := registry.Get("claude")
	initialMax := limiter1.MaxTokens()

	// Update config
	registry.SetConfig("claude", RateLimiterConfig{
		MaxTokens:  20,
		RefillRate: 2,
	})

	// Get limiter again - should be new instance with new config
	limiter2 := registry.Get("claude")
	if limiter2.MaxTokens() != 20 {
		t.Errorf("MaxTokens = %v, want 20", limiter2.MaxTokens())
	}
	if limiter2.MaxTokens() == initialMax {
		t.Error("config update should change MaxTokens")
	}
}

func TestRateLimiterRegistry_SetConfigUpdatesLimiter(t *testing.T) {
	registry := NewRateLimiterRegistry()

	registry.SetConfig("test-agent", RateLimiterConfig{
		MaxTokens:  2,
		RefillRate: 0.1,
	})

	limiter := registry.Get("test-agent")
	initial := limiter.RefillRate()

	registry.SetConfig("test-agent", RateLimiterConfig{
		MaxTokens:  4,
		RefillRate: 2,
	})

	if limiter.RefillRate() == initial {
		t.Error("SetConfig should update existing limiter")
	}
	if limiter.MaxTokens() != 4 {
		t.Errorf("MaxTokens = %v, want 4", limiter.MaxTokens())
	}
}

func TestRateLimiterRegistry_Status(t *testing.T) {
	registry := NewRateLimiterRegistry()

	// Initialize some limiters
	registry.Get("claude")
	registry.Get("gemini")

	status := registry.Status()
	if len(status) != 2 {
		t.Errorf("len(Status) = %d, want 2", len(status))
	}

	claudeStatus, ok := status["claude"]
	if !ok {
		t.Fatal("status should contain claude")
	}
	if claudeStatus.MaxTokens != 5 {
		t.Errorf("claude MaxTokens = %v, want 5", claudeStatus.MaxTokens)
	}
}

func TestRateLimiterRegistry_Wait(t *testing.T) {
	registry := NewRateLimiterRegistry()
	registry.SetConfig("test-agent", RateLimiterConfig{
		MaxTokens:  1,
		RefillRate: 2, // 1 token per 500ms
	})

	ctx := context.Background()

	start := time.Now()
	if err := registry.Wait(ctx, "test-agent"); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if err := registry.Wait(ctx, "test-agent"); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 300*time.Millisecond {
		t.Errorf("Wait should block for refill, elapsed = %v", elapsed)
	}
}

func TestRateLimiterRegistry_Allow(t *testing.T) {
	registry := NewRateLimiterRegistry()
	registry.SetConfig("test-agent", RateLimiterConfig{
		MaxTokens:  2,
		RefillRate: 0.01,
	})

	if !registry.Allow("test-agent") {
		t.Error("first Allow should succeed")
	}
	if !registry.Allow("test-agent") {
		t.Error("second Allow should succeed")
	}
	if registry.Allow("test-agent") {
		t.Error("third Allow should fail")
	}
}

func TestRateLimiterRegistry_Reset(t *testing.T) {
	registry := NewRateLimiterRegistry()

	registry.Get("claude")
	if len(registry.Status()) == 0 {
		t.Fatal("expected limiter status before reset")
	}

	registry.Reset()
	if len(registry.Status()) != 0 {
		t.Error("Reset should clear limiters")
	}
}

func TestRateLimiterRegistry_List(t *testing.T) {
	registry := NewRateLimiterRegistry()

	adapters := registry.List()
	if len(adapters) != 5 {
		t.Errorf("len(List) = %d, want 5", len(adapters))
	}

	// Check expected adapters
	expected := map[string]bool{
		"claude": true, "gemini": true, "codex": true,
		"copilot": true, "opencode": true,
	}
	for _, name := range adapters {
		if !expected[name] {
			t.Errorf("unexpected adapter: %s", name)
		}
	}
}

func TestAdaptiveRateLimiter_Success(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1.0,
	}
	limiter := NewAdaptiveRateLimiter(cfg)

	initialRate := limiter.CurrentRefillRate()

	// Record 5 successes (threshold for rate increase)
	for i := 0; i < 5; i++ {
		limiter.RecordSuccess()
	}

	newRate := limiter.CurrentRefillRate()
	if newRate <= initialRate {
		t.Errorf("rate should increase after 5 successes: %v -> %v", initialRate, newRate)
	}
}

func TestAdaptiveRateLimiter_Error(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1.0,
	}
	limiter := NewAdaptiveRateLimiter(cfg)

	initialRate := limiter.CurrentRefillRate()

	// Record an error (should immediately decrease rate)
	limiter.RecordError()

	newRate := limiter.CurrentRefillRate()
	if newRate >= initialRate {
		t.Errorf("rate should decrease after error: %v -> %v", initialRate, newRate)
	}

	// Rate should be approximately half
	expected := initialRate * 0.5
	if newRate < expected*0.9 || newRate > expected*1.1 {
		t.Errorf("rate = %v, want ~%v", newRate, expected)
	}
}

func TestAdaptiveRateLimiter_MinRate(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1.0,
	}
	limiter := NewAdaptiveRateLimiter(cfg)

	// Record many errors - should not go below min rate
	for i := 0; i < 20; i++ {
		limiter.RecordError()
	}

	rate := limiter.CurrentRefillRate()
	minRate := cfg.RefillRate * 0.1 // minRefillRate
	if rate < minRate {
		t.Errorf("rate = %v, should not go below min = %v", rate, minRate)
	}
}

func TestAdaptiveRateLimiter_MaxRate(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1.0,
	}
	limiter := NewAdaptiveRateLimiter(cfg)

	// Record many successes - should not go above max rate
	for i := 0; i < 100; i++ {
		limiter.RecordSuccess()
	}

	rate := limiter.CurrentRefillRate()
	maxRate := cfg.RefillRate * 2 // maxRefillRate
	if rate > maxRate {
		t.Errorf("rate = %v, should not go above max = %v", rate, maxRate)
	}
}

func TestAdaptiveRateLimiter_SuccessResetsErrors(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  10,
		RefillRate: 1.0,
	}
	limiter := NewAdaptiveRateLimiter(cfg)

	// Record some errors
	limiter.RecordError()
	limiter.RecordError()
	rateAfterErrors := limiter.CurrentRefillRate()

	// Record success (resets error counter)
	limiter.RecordSuccess()

	// Record 4 more successes (need 5 total for increase)
	for i := 0; i < 4; i++ {
		limiter.RecordSuccess()
	}

	rateAfterSuccesses := limiter.CurrentRefillRate()
	if rateAfterSuccesses <= rateAfterErrors {
		t.Error("rate should increase after successful recovery")
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.MaxTokens != 10 {
		t.Errorf("MaxTokens = %v, want 10", cfg.MaxTokens)
	}
	if cfg.RefillRate != 1 {
		t.Errorf("RefillRate = %v, want 1", cfg.RefillRate)
	}
}

func TestRateLimiter_MaxTokensCap(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  5,
		RefillRate: 100, // Very fast
	}
	limiter := NewRateLimiter(cfg)

	// Wait for potential over-refill
	time.Sleep(100 * time.Millisecond)

	// Available should not exceed MaxTokens
	available := limiter.Available()
	if available > cfg.MaxTokens {
		t.Errorf("Available = %v, should not exceed MaxTokens = %v", available, cfg.MaxTokens)
	}
}
