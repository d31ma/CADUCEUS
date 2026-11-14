package realtime

import (
	"encoding/base64"
	"slices"
)

type SignalCheck struct{}

func (check SignalCheck) Signal(message ClientMessage) error {
	return check.first(
		check.request(message.RequestId),
		check.targets(message.ToDeviceIds),
		check.kind(message.Kind),
		check.payload(message.Ciphertext, SignalLimits{}.MaximumCiphertextBytes(), InvalidSignal{}.CiphertextSize()),
	)
}

func (check SignalCheck) Pairing(message ClientMessage) error {
	return check.first(check.request(message.RequestId), check.pairingId(message.PairingId))
}

func (check SignalCheck) CallCreate(message ClientMessage) error {
	return check.first(check.request(message.RequestId), check.media(message.Media))
}

func (check SignalCheck) Call(message ClientMessage) error {
	return check.first(check.request(message.RequestId), check.callId(message.CallId))
}

func (check SignalCheck) Ring(message ClientMessage) error {
	return check.first(
		check.Call(message),
		check.people(message.ToPersonIds, 1, InvalidCall{}.RingCount()),
		check.payload(message.Invite, SignalLimits{}.MaximumInviteBytes(), InvalidCall{}.InviteSize()),
	)
}

func (check SignalCheck) RingEnd(message ClientMessage) error {
	return check.first(check.Call(message), check.people(message.Missed, 0, InvalidCall{}.MissedCount()))
}

func (check SignalCheck) Device(connection Connection) error {
	if connection.DeviceId == "" {
		return InvalidSignal{}.DeviceMissing()
	}
	return nil
}

func (check SignalCheck) Linked(connection Connection) error {
	if !connection.Linked {
		return CallRefusal{}.DeviceNotLinked()
	}
	return nil
}

func (SignalCheck) first(results ...error) error {
	for _, result := range results {
		if result != nil {
			return result
		}
	}
	return nil
}

func (SignalCheck) request(requestId string) error {
	if len(requestId) > (SignalLimits{}).MaximumIdLength() {
		return InvalidSignal{}.RequestIdInvalid()
	}
	return nil
}

func (check SignalCheck) targets(deviceIds []string) error {
	if len(deviceIds) == 0 || len(deviceIds) > (SignalLimits{}).MaximumTargets() || slices.ContainsFunc(deviceIds, check.badId) {
		return InvalidSignal{}.TargetCount()
	}
	return nil
}

func (check SignalCheck) people(personIds []string, minimum int, refusal SignalRefusal) error {
	if len(personIds) < minimum || len(personIds) > (SignalLimits{}).MaximumRingPeople() || slices.ContainsFunc(personIds, check.badId) {
		return refusal
	}
	return nil
}

func (SignalCheck) badId(id string) bool {
	return !IdText{}.Fits(id, 1)
}

func (SignalCheck) kind(kind string) error {
	if !slices.Contains([]string{"rtc", "call", "linking"}, kind) {
		return InvalidSignal{}.UnknownKind()
	}
	return nil
}

func (SignalCheck) payload(text string, maximum int, refusal SignalRefusal) error {
	decoded, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil || len(decoded) == 0 || len(decoded) > maximum {
		return refusal
	}
	return nil
}

func (SignalCheck) pairingId(pairingId string) error {
	if !(IdText{}).Fits(pairingId, (SignalLimits{}).MinimumPairingIdLength()) {
		return InvalidCall{}.PairingIdInvalid()
	}
	return nil
}

func (SignalCheck) media(media string) error {
	if media != "audio" && media != "video" {
		return InvalidCall{}.UnknownMedia()
	}
	return nil
}

func (SignalCheck) callId(callId string) error {
	if !(IdText{}).Fits(callId, 1) {
		return MissingTarget{}.CallNotFound()
	}
	return nil
}
