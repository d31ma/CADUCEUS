package dkim

import "fmt"

type UnsafeSecretName struct {
	Name string
}

func (unsafe UnsafeSecretName) Error() string {
	return fmt.Sprintf("dkim: secret name %q is not safe", unsafe.Name)
}
