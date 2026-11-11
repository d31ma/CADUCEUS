package webpush

import "fmt"

type PayloadTooLarge struct {
	Size  int
	Limit int
}

func (tooLarge PayloadTooLarge) Error() string {
	return fmt.Sprintf("the push payload has %d bytes; the limit is %d bytes", tooLarge.Size, tooLarge.Limit)
}
