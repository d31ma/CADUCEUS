package inbound

import "mesail.dev/packages/core"

type Unavailability struct{}

func (Unavailability) Of(cause error) error {
	return &core.Unavailable{Message: "the messaging storage is not available", Cause: cause}
}

func (Unavailability) Missing(what string) error {
	return &core.Unavailable{Message: what + " is not stored yet"}
}
