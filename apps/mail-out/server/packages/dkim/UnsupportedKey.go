package dkim

import "fmt"

type UnsupportedKey struct {
	Reason string
}

func (unsupported UnsupportedKey) Error() string {
	return fmt.Sprintf("dkim: unsupported key: %s", unsupported.Reason)
}
