package dkim

import "fmt"

type MalformedMessage struct {
	Reason string
}

func (malformed MalformedMessage) Error() string {
	return fmt.Sprintf("dkim: malformed message: %s", malformed.Reason)
}
