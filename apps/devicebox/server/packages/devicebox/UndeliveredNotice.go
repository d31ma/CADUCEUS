package devicebox

import (
	"time"

	"mesail.dev/packages/oidc"
)

type UndeliveredNotice struct {
	parts ExpiryParts
	now   time.Time
}

func NewUndeliveredNotice(parts ExpiryParts, now time.Time) UndeliveredNotice {
	return UndeliveredNotice{parts: parts, now: now}
}

func (notice UndeliveredNotice) Leave(domainId string, expired Slip) (int, error) {
	put := notice.put(expired)
	owners := map[string]string{expired.SenderDeviceId: expired.SenderPersonId}
	fields, err := notice.parts.Staging.Staged(domainId, Delivery{EnvelopeId: put.EnvelopeId, Kind: put.Kind, DeviceIds: put.ToDeviceIds, Owners: owners})
	if err != nil {
		return 0, err
	}
	bodyId, err := notice.parts.Store.InsertBody(domainId, EnvelopeParts{}.Body(domainId, put, notice.now, fields))
	if err != nil {
		return 0, err
	}
	slip := EnvelopeParts{}.Slip(oidc.Caller{DomainId: domainId, MemberId: expired.SenderPersonId}, put, notice.now)
	slip.BodyId = bodyId
	_, err = notice.parts.Store.InsertSlip(domainId, slip)
	notice.parts.Desk.Drain(Names{}.Bodies(domainId), bodyId)
	return notice.Cost(), err
}

func (UndeliveredNotice) Cost() int {
	return 3
}

func (UndeliveredNotice) put(expired Slip) CheckedPut {
	return CheckedPut{
		EnvelopeId:     expired.EnvelopeId,
		SenderDeviceId: expired.SenderDeviceId,
		ToDeviceIds:    []string{expired.SenderDeviceId},
		Kind:           Formats{}.Undelivered(),
	}
}
