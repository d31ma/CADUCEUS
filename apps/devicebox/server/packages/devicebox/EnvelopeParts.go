package devicebox

import (
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type EnvelopeParts struct{}

func (parts EnvelopeParts) Slip(caller oidc.Caller, put CheckedPut, now time.Time) Slip {
	return Slip{
		EnvelopeId:     put.EnvelopeId,
		DomainId:       caller.DomainId,
		SenderDeviceId: put.SenderDeviceId,
		SenderPersonId: caller.MemberId,
		Kind:           put.Kind,
		StoredAt:       now.Format(time.RFC3339Nano),
		StoredUs:       now.UnixMicro(),
		ExpiresUs:      now.Add(Limits{}.Lifetime()).UnixMicro(),
		ContentHash:    ContentHash{}.Of(put),
		Waiting:        parts.waiting(parts.Keys(put.ToDeviceIds)),
	}
}

func (EnvelopeParts) Body(domainId string, put CheckedPut, now time.Time, fields outbox.Fields) Body {
	patch := OutboxPatch{}.From(fields)
	return Body{
		EnvelopeId:    put.EnvelopeId,
		DomainId:      domainId,
		Ciphertext:    put.Ciphertext,
		ExpiresUs:     now.Add(Limits{}.Lifetime()).UnixMicro(),
		PendingEvents: patch.PendingEvents,
		DeadEvents:    patch.DeadEvents,
		OutboxPending: patch.OutboxPending,
		OutboxDead:    patch.OutboxDead,
		OutboxWaiting: patch.OutboxWaiting,
		OutboxHasDead: patch.OutboxHasDead,
	}
}

func (EnvelopeParts) Receipt(slip Slip) EnvelopeReceipt {
	expires := time.UnixMicro(slip.ExpiresUs).UTC()
	return EnvelopeReceipt{EnvelopeId: slip.EnvelopeId, ExpiresAt: expires.Format(time.RFC3339)}
}

func (EnvelopeParts) Keys(deviceIds []string) []string {
	keys := []string{}
	for _, deviceId := range deviceIds {
		keys = append(keys, DeviceKey{}.Of(deviceId))
	}
	return keys
}

func (EnvelopeParts) Deltas(deviceKeys []string, delta int) map[string]int {
	deltas := map[string]int{}
	for _, deviceKey := range deviceKeys {
		deltas[deviceKey] += delta
	}
	return deltas
}

func (EnvelopeParts) waiting(deviceKeys []string) map[string]bool {
	waiting := map[string]bool{}
	for _, deviceKey := range deviceKeys {
		waiting[deviceKey] = true
	}
	return waiting
}
