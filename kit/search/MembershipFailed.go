package search

import "fmt"

type MembershipFailed struct {
	Service string
	Status  int
}

func (failure MembershipFailed) Error() string {
	return fmt.Sprintf("the %s service answered status %d", failure.Service, failure.Status)
}
