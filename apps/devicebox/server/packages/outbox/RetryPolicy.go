package outbox

import "time"

type RetryPolicy struct {
	baseDelay   time.Duration
	maxDelay    time.Duration
	maxAttempts int
}

func NewRetryPolicy(settings RetrySettings) RetryPolicy {
	complete := settings.withDefaults()
	return RetryPolicy{
		baseDelay:   time.Duration(complete.BaseDelaySeconds) * time.Second,
		maxDelay:    time.Duration(complete.MaxDelaySeconds) * time.Second,
		maxAttempts: complete.MaxAttempts,
	}
}

func (policy RetryPolicy) Delay(attempts int) time.Duration {
	doublings := min(max(attempts-1, 0), policy.maxDoublings())
	return min(policy.baseDelay<<doublings, policy.maxDelay)
}

func (policy RetryPolicy) Exhausted(attempts int) bool {
	return attempts >= policy.maxAttempts
}

func (RetryPolicy) maxDoublings() int {
	return 20
}
