package blobs

import "fmt"

type SizeMismatch struct {
	Expected int64
	Received int64
}

func (mismatch SizeMismatch) Error() string {
	return fmt.Sprintf("expected %d bytes but received %d", mismatch.Expected, mismatch.Received)
}
