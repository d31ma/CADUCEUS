package devicebox

import (
	"time"

	"mesail.dev/packages/oidc"
)

type Mailbox struct {
	callers   Callers
	ownership Ownership
	store     Store
	counts    BoxCounts
	staging   EventStaging
	desk      OutboxDesk
	now       func() time.Time
}

func (mailbox Mailbox) Put(authorization string, request PutRequest) (EnvelopeReceipt, error) {
	caller, err := mailbox.callers.Member(authorization)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	put, err := PutCheck{}.Checked(request)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	owners, err := mailbox.ownership.ForPut(caller, put)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	err = mailbox.store.EnsureDomain(caller.DomainId)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	existing, found, err := mailbox.store.SlipByEnvelope(caller.DomainId, put.EnvelopeId)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	return mailbox.putOnce(caller, put, owners, existing, found)
}

func (mailbox Mailbox) List(authorization string, deviceId string, after string, limit string) (EnvelopePage, error) {
	caller, err := mailbox.callers.Member(authorization)
	if err != nil {
		return EnvelopePage{}, err
	}
	afterUs, err := PageCheck{}.After(after)
	if err != nil {
		return EnvelopePage{}, err
	}
	size, err := PageCheck{}.Limit(limit)
	if err != nil {
		return EnvelopePage{}, err
	}
	err = mailbox.ownership.ForBox(caller, deviceId)
	if err != nil {
		return EnvelopePage{}, err
	}
	rows, err := mailbox.store.Waiting(caller.DomainId, DeviceKey{}.Of(deviceId), afterUs, size)
	if err != nil {
		return EnvelopePage{}, err
	}
	return NewPageBuilder(mailbox.store).Page(caller.DomainId, rows, size)
}

func (mailbox Mailbox) Ack(authorization string, deviceId string, envelopeIds []string) error {
	caller, err := mailbox.callers.Member(authorization)
	if err != nil {
		return err
	}
	checked, err := AckCheck{}.Checked(envelopeIds)
	if err != nil {
		return err
	}
	err = mailbox.ownership.ForBox(caller, deviceId)
	if err != nil {
		return err
	}
	deviceKey := DeviceKey{}.Of(deviceId)
	removed, err := NewAckRun(mailbox.store).Remove(caller.DomainId, deviceKey, checked)
	if err != nil {
		return err
	}
	return mailbox.lowerCount(caller.DomainId, deviceKey, removed)
}

func (mailbox Mailbox) putOnce(caller oidc.Caller, put CheckedPut, owners map[string]string, existing Slip, found bool) (EnvelopeReceipt, error) {
	if found {
		return mailbox.repeated(existing, put)
	}
	return mailbox.stored(caller, put, owners)
}

func (mailbox Mailbox) repeated(existing Slip, put CheckedPut) (EnvelopeReceipt, error) {
	if existing.ContentHash != (ContentHash{}).Of(put) {
		return EnvelopeReceipt{}, Duplicate{}.EnvelopeId()
	}
	return EnvelopeParts{}.Receipt(existing), nil
}

func (mailbox Mailbox) stored(caller oidc.Caller, put CheckedPut, owners map[string]string) (EnvelopeReceipt, error) {
	deviceKeys := EnvelopeParts{}.Keys(put.ToDeviceIds)
	err := mailbox.counts.Room(caller.DomainId, deviceKeys)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	delivery := Delivery{EnvelopeId: put.EnvelopeId, Kind: put.Kind, DeviceIds: put.ToDeviceIds, Owners: owners}
	fields, err := mailbox.staging.Staged(caller.DomainId, delivery)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	now := mailbox.now().UTC()
	slip := EnvelopeParts{}.Slip(caller, put, now)
	bodyId, err := mailbox.store.InsertBody(caller.DomainId, EnvelopeParts{}.Body(caller.DomainId, put, now, fields))
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	slip.BodyId = bodyId
	_, err = mailbox.store.InsertSlip(caller.DomainId, slip)
	if err != nil {
		return EnvelopeReceipt{}, err
	}
	_, err = mailbox.counts.Add(caller.DomainId, EnvelopeParts{}.Deltas(deviceKeys, 1))
	mailbox.desk.Drain(Names{}.Bodies(caller.DomainId), bodyId)
	return EnvelopeParts{}.Receipt(slip), err
}

func (mailbox Mailbox) lowerCount(domainId string, deviceKey string, removed int) error {
	if removed == 0 {
		return nil
	}
	_, err := mailbox.counts.Add(domainId, map[string]int{deviceKey: -removed})
	return err
}
