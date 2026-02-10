package issues

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func TestIsZeroResilienceConfig_Zero(t *testing.T) {
	cfg := config.LLMResilienceConfig{}
	if !isZeroResilienceConfig(cfg) {
		t.Error("zero config should return true")
	}
}

func TestIsZeroResilienceConfig_Enabled(t *testing.T) {
	cfg := config.LLMResilienceConfig{Enabled: true}
	if isZeroResilienceConfig(cfg) {
		t.Error("enabled config should return false")
	}
}

func TestIsZeroResilienceConfig_WithRetries(t *testing.T) {
	cfg := config.LLMResilienceConfig{MaxRetries: 3}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with retries should return false")
	}
}

func TestIsZeroResilienceConfig_WithBackoff(t *testing.T) {
	cfg := config.LLMResilienceConfig{InitialBackoff: "1s"}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with initial backoff should return false")
	}
}

func TestIsZeroResilienceConfig_WithMaxBackoff(t *testing.T) {
	cfg := config.LLMResilienceConfig{MaxBackoff: "30s"}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with max backoff should return false")
	}
}

func TestIsZeroResilienceConfig_WithMultiplier(t *testing.T) {
	cfg := config.LLMResilienceConfig{BackoffMultiplier: 2.0}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with multiplier should return false")
	}
}

func TestIsZeroResilienceConfig_WithThreshold(t *testing.T) {
	cfg := config.LLMResilienceConfig{FailureThreshold: 5}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with threshold should return false")
	}
}

func TestIsZeroResilienceConfig_WithResetTimeout(t *testing.T) {
	cfg := config.LLMResilienceConfig{ResetTimeout: "5m"}
	if isZeroResilienceConfig(cfg) {
		t.Error("config with reset timeout should return false")
	}
}

func TestResolveExecuteTimeout_NoDeadline(t *testing.T) {
	fallback := 10 * time.Minute
	got := resolveExecuteTimeout(time.Time{}, false, fallback)
	if got != fallback {
		t.Errorf("got %v, want %v", got, fallback)
	}
}

func TestResolveExecuteTimeout_DeadlineFarFuture(t *testing.T) {
	fallback := 10 * time.Minute
	deadline := time.Now().Add(1 * time.Hour) // 1h > 10min
	got := resolveExecuteTimeout(deadline, true, fallback)
	if got != fallback {
		t.Errorf("got %v, want %v (deadline is farther than fallback)", got, fallback)
	}
}

func TestResolveExecuteTimeout_DeadlineSoon(t *testing.T) {
	fallback := 10 * time.Minute
	deadline := time.Now().Add(30 * time.Second) // 30s < 10min
	got := resolveExecuteTimeout(deadline, true, fallback)
	if got >= fallback {
		t.Errorf("got %v, should be less than fallback %v", got, fallback)
	}
	if got <= 0 {
		t.Errorf("got %v, should be positive", got)
	}
}

func TestResolveExecuteTimeout_DeadlinePassed(t *testing.T) {
	fallback := 10 * time.Minute
	deadline := time.Now().Add(-1 * time.Second) // past deadline
	got := resolveExecuteTimeout(deadline, true, fallback)
	if got != fallback {
		t.Errorf("got %v, want %v (past deadline should use fallback)", got, fallback)
	}
}
