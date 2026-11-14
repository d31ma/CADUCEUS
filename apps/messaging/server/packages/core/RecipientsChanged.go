package core

import "strings"

type RecipientsChanged struct {
	Addresses   []string
	ErrorNumber int
}

func (RecipientsChanged) Of(addresses []string) *RecipientsChanged {
	return &RecipientsChanged{Addresses: addresses, ErrorNumber: 1547}
}

func (failure *RecipientsChanged) Error() string {
	return "the recipients changed: " + strings.Join(failure.Addresses, ", ")
}

func (failure *RecipientsChanged) Number() int {
	return failure.ErrorNumber
}
