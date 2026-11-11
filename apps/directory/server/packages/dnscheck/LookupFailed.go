package dnscheck

import "fmt"

type LookupFailed struct {
	Name   string
	Status int
}

func (failed LookupFailed) Error() string {
	return fmt.Sprintf("dns lookup of %q failed with status %d", failed.Name, failed.Status)
}
