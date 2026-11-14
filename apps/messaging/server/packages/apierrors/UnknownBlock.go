package apierrors

import "fmt"

type UnknownBlock struct {
	First int
}

func (unknown UnknownBlock) Error() string {
	return fmt.Sprintf("ERRORS.md has no block that starts at MS%04d", unknown.First)
}
