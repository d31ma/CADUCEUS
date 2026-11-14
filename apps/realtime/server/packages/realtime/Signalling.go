package realtime

import (
	"errors"
	"slices"

	"mesail.dev/packages/apierrors"
)

type Signalling struct {
	Signals  Signals
	Pairings Pairings
	Calls    Calls
	Pushes   ConnectionGateway
}

func (signalling Signalling) Handles(messageType string) bool {
	return slices.Contains(signalling.types(), messageType)
}

func (signalling Signalling) Handle(connection Connection, message ClientMessage) error {
	return signalling.answered(connection, message, signalling.dispatched(connection, message))
}

func (signalling Signalling) dispatched(connection Connection, message ClientMessage) error {
	types := FrameTypes{}
	switch message.Type {
	case types.Signal():
		return signalling.Signals.Send(connection, message)
	case types.PairingOpen():
		return signalling.Pairings.Open(connection, message)
	case types.PairingJoin():
		return signalling.Pairings.Join(connection, message)
	case types.CallCreate():
		return signalling.Calls.Create(connection, message)
	case types.CallJoin():
		return signalling.Calls.Join(connection, message)
	case types.CallRing():
		return signalling.Calls.Ring(connection, message)
	case types.CallRingEnd():
		return signalling.Calls.EndRinging(connection, message)
	}
	return signalling.Calls.Leave(connection, message)
}

func (Signalling) types() []string {
	types := FrameTypes{}
	return []string{types.Signal(), types.PairingOpen(), types.PairingJoin(), types.CallCreate(), types.CallJoin(), types.CallRing(), types.CallRingEnd(), types.CallLeave()}
}

func (signalling Signalling) answered(connection Connection, message ClientMessage, handled error) error {
	if handled == nil || errors.As(handled, &Gone{}) {
		return nil
	}
	refusal := signalling.refusal(handled)
	frame := RefusedFrame{Type: FrameTypes{}.Refused(), RequestType: message.Type, RequestId: message.RequestId, State: refusal.Code, Field: refusal.Field, Message: apierrors.Message{}.Format(refusal.Number(), refusal.Text)}
	return Gateway{}.unlessGone(signalling.Pushes.Send(connection.ConnectionId, frame))
}

func (Signalling) refusal(handled error) SignalRefusal {
	refusal := SignalRefusal{}
	if !errors.As(handled, &refusal) {
		return CallRefusal{}.Unexpected()
	}
	return refusal
}
