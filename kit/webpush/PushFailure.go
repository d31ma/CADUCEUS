package webpush

import "fmt"

type PushFailure struct {
	Status int
	Detail string
}

func (failure PushFailure) Error() string {
	return fmt.Sprintf("the push service did not accept the message (status %d): %s", failure.Status, failure.Detail)
}
