package dkim

import "fmt"

type MissingKey struct {
	Domain   string
	Selector string
}

func (missing MissingKey) Error() string {
	return fmt.Sprintf("dkim: no key for domain %q and selector %q", missing.Domain, missing.Selector)
}
