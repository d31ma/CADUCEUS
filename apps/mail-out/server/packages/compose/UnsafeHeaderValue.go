package compose

import "fmt"

type UnsafeHeaderValue struct {
	Field string
}

func (unsafe UnsafeHeaderValue) Error() string {
	return fmt.Sprintf("the %s header value contains a line break", unsafe.Field)
}
