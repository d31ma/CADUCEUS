package blobs

import "fmt"

type TooLarge struct {
	Limit int64
}

func (large TooLarge) Error() string {
	return fmt.Sprintf("the blob is larger than %d bytes", large.Limit)
}
