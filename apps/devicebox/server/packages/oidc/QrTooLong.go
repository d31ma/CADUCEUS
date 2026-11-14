package oidc

import "fmt"

type QrTooLong struct {
	Length   int
	Capacity int
}

func (failure QrTooLong) Error() string {
	return fmt.Sprintf("qr: text is %d bytes but a version 10-M code holds at most %d bytes", failure.Length, failure.Capacity)
}
