package dkim

import (
	"fmt"
	"io/fs"
)

type MissingSecret struct {
	Name string
}

func (missing MissingSecret) Error() string {
	return fmt.Sprintf("dkim: secret %q does not exist", missing.Name)
}

func (missing MissingSecret) Unwrap() error {
	return fs.ErrNotExist
}
