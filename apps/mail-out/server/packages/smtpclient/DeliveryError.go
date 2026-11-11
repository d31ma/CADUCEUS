package smtpclient

import "fmt"

type DeliveryError struct {
	Reply     Reply
	Host      string
	Permanent bool
	Cause     error
}

func (failure DeliveryError) Error() string {
	return fmt.Sprintf("smtp delivery via %s failed: %s", failure.Host, failure.detail())
}

func (failure DeliveryError) Unwrap() error {
	return failure.Cause
}

func (failure DeliveryError) detail() string {
	if failure.Reply.Code == 0 {
		return failure.Reply.Text
	}
	return fmt.Sprintf("%d %s %s", failure.Reply.Code, failure.Reply.Enhanced, failure.Reply.Text)
}
