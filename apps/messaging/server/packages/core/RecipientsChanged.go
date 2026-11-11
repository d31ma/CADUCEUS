package core

import "strings"

type RecipientsChanged struct {
	Addresses []string
}

func (failure *RecipientsChanged) Error() string {
	return "the recipients changed: " + strings.Join(failure.Addresses, ", ")
}
