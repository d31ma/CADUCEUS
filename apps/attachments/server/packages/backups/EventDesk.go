package backups

import (
	"mesail.dev/packages/attachments"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type EventDesk struct {
	Gate    attachments.CallerGate
	Seen    events.SeenStore
	Removal Removal
}

func (desk EventDesk) Receive(authorization string, envelope events.Envelope) (events.Receipt, error) {
	caller, err := desk.identify(authorization)
	if err != nil {
		return events.Receipt{}, err
	}
	err = desk.checkDomain(caller, envelope)
	if err != nil {
		return events.Receipt{}, err
	}
	return events.NewReceiver(events.Services{}.Attachments(), desk.Seen, desk.Removal).Receive(caller, envelope)
}

func (desk EventDesk) identify(authorization string) (oidc.Caller, error) {
	caller, err := desk.Gate.Identify(authorization)
	if err != nil {
		return oidc.Caller{}, attachments.Unauthenticated{}
	}
	return caller, nil
}

func (desk EventDesk) checkDomain(caller oidc.Caller, envelope events.Envelope) error {
	if caller.DomainId != envelope.DomainId && caller.DomainId != desk.installWide() {
		return OtherDomain{}
	}
	return nil
}

func (EventDesk) installWide() string {
	return "*"
}
