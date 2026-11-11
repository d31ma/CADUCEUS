package backend

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type RetryPolicy struct {
	Delays []time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second}}
}

func (policy RetryPolicy) Retryable(failure error) bool {
	status := UnexpectedStatus{}
	if errors.As(failure, &status) {
		return policy.retryableStatus(status.Status)
	}
	return failure != nil && !errors.Is(failure, context.Canceled)
}

func (RetryPolicy) retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func (RetryPolicy) Wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
