package inbound

import "mesail.dev/packages/core"

type Unavailability struct{}

func (Unavailability) Of(cause error) error {
	return core.Unavailable{}.InboundStorage(cause)
}

func (Unavailability) Missing(what string) error {
	return core.Unavailable{}.NotStoredYet(what)
}
