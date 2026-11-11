package outbox

import "fmt"

type DeliveryFailure struct {
	Subscriber string
	Status     int
	Permanent  bool
	Detail     string
}

func (failure DeliveryFailure) Error() string {
	return fmt.Sprintf("delivery to %s failed (status %d, permanent %t): %s", failure.Subscriber, failure.Status, failure.Permanent, failure.Detail)
}
