package realtime

import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type EventDesk struct {
	Gate    oidc.Gate
	Seen    SeenBook
	Members MemberBook
	FanOut  FanOut
}

func (desk EventDesk) Receive(authorization string, envelope events.Envelope) (events.Receipt, error) {
	return events.Receipt{}, nil
}

func (desk EventDesk) Handle(envelope events.Envelope) error {
	return nil
}
