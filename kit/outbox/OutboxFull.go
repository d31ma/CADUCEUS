package outbox

import "fmt"

type OutboxFull struct {
	Bytes int
	Limit int
}

func (failure OutboxFull) Error() string {
	return fmt.Sprintf("the document's waiting events would take %d bytes, over the %d byte document cap; try again later", failure.Bytes, failure.Limit)
}
