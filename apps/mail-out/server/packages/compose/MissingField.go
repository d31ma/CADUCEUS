package compose

import "fmt"

type MissingField struct {
	Field string
}

func (missing MissingField) Error() string {
	return fmt.Sprintf("the %s field is required", missing.Field)
}
