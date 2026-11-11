package dkim

import "fmt"

type InvalidKeyName struct {
	Domain   string
	Selector string
}

func (invalid InvalidKeyName) Error() string {
	return fmt.Sprintf("dkim: domain %q or selector %q is not a valid DNS label list", invalid.Domain, invalid.Selector)
}
