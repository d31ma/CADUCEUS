package outbox

import "errors"

type pushAttempt struct {
	subscriber string
	skipped    bool
	err        error
}

func (attempt pushAttempt) kind() string {
	kinds := attemptKind{}
	switch {
	case attempt.skipped:
		return kinds.skipped()
	case attempt.err == nil:
		return kinds.delivered()
	case attempt.permanent():
		return kinds.dead()
	}
	return kinds.retry()
}

func (attempt pushAttempt) permanent() bool {
	failure := DeliveryFailure{}
	errors.As(attempt.err, &failure)
	return failure.Permanent
}
