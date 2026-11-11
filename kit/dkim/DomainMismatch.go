package dkim

import "fmt"

type DomainMismatch struct {
	From string
	Key  string
}

func (mismatch DomainMismatch) Error() string {
	return fmt.Sprintf("dkim: the From address %q is not in the key domain %q", mismatch.From, mismatch.Key)
}
