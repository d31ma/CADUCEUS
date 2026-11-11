package outbox

import "fmt"

type NotPublished struct {
	Type  string
	Count int
}

func (failure NotPublished) Error() string {
	return fmt.Sprintf("expected exactly one staged %s event, found %d", failure.Type, failure.Count)
}
