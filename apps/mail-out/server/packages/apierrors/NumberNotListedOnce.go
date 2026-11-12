package apierrors

import "fmt"

type NumberNotListedOnce struct {
	Number int
	Times  int
}

func (notListed NumberNotListedOnce) Error() string {
	return fmt.Sprintf("MS%04d is listed %d times in ERRORS.md; it must be listed once", notListed.Number, notListed.Times)
}
