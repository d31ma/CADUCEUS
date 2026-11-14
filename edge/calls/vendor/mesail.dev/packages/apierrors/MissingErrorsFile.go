package apierrors

import "fmt"

type MissingErrorsFile struct {
	From string
}

func (missing MissingErrorsFile) Error() string {
	return fmt.Sprintf("no docs/mesail/ERRORS.md in %s or any folder above it", missing.From)
}
