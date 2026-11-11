package outbox

import (
	"testing"
	"time"
)

func TestRetryPolicyDoublesTheDelayUpToTheMaximum(t *testing.T) {
	policy := NewRetryPolicy(RetrySettings{})

	if policy.Delay(1) != 5*time.Second || policy.Delay(2) != 10*time.Second || policy.Delay(4) != 40*time.Second {
		t.Fatalf("delays %v %v %v", policy.Delay(1), policy.Delay(2), policy.Delay(4))
	}
	if policy.Delay(9) != 900*time.Second || policy.Delay(500) != 900*time.Second {
		t.Fatalf("capped delays %v %v", policy.Delay(9), policy.Delay(500))
	}
	if policy.Delay(0) != 5*time.Second {
		t.Fatalf("delay before any attempt %v", policy.Delay(0))
	}
}

func TestRetryPolicyUsesConfiguredValues(t *testing.T) {
	policy := NewRetryPolicy(RetrySettings{BaseDelaySeconds: 1, MaxDelaySeconds: 3, MaxAttempts: 2})

	if policy.Delay(1) != time.Second || policy.Delay(3) != 3*time.Second {
		t.Fatalf("delays %v %v", policy.Delay(1), policy.Delay(3))
	}
	if policy.Exhausted(1) || !policy.Exhausted(2) {
		t.Fatal("two attempts should exhaust the policy")
	}
}

func TestRetryPolicyDefaultsToTwelveAttempts(t *testing.T) {
	policy := NewRetryPolicy(RetrySettings{})

	if policy.Exhausted(11) || !policy.Exhausted(12) {
		t.Fatal("twelve attempts should exhaust the default policy")
	}
}
