package realtime

import (
	"crypto/rand"
	"slices"
	"time"
)

type Calls struct {
	Book        CallBook
	Seats       SeatBook
	Starts      CallStartBook
	Signer      CallSigner
	Connections ConnectionBook
	Pushes      ConnectionGateway
	Notify      CallPushes
	Log         CallPushLog
	Now         func() time.Time
}

func (calls Calls) Create(connection Connection, message ClientMessage) error {
	err := calls.member(connection, SignalCheck{}.CallCreate(message))
	if err != nil {
		return err
	}
	err = calls.signable(message.Group)
	if err != nil {
		return err
	}
	err = calls.Starts.Allow(connection.DomainId, connection.DeviceId)
	if err != nil {
		return err
	}
	record := CallRecord{CallId: calls.newId(), Group: message.Group, Media: message.Media, CreatedByDevice: connection.DeviceId, CreatedByPerson: connection.MemberId, CreatedAt: calls.now(), LastChangeAt: calls.now()}
	err = calls.Book.Create(connection.DomainId, record)
	if err != nil {
		return err
	}
	err = calls.Seats.Add(connection.DomainId, calls.seat(record.CallId, connection))
	if err != nil {
		return err
	}
	return calls.answer(connection, message, record, FrameTypes{}.CallCreated())
}

func (calls Calls) Join(connection Connection, message ClientMessage) error {
	err := calls.member(connection, SignalCheck{}.Call(message))
	if err != nil {
		return err
	}
	record, err := calls.Book.Find(connection.DomainId, message.CallId)
	if err != nil {
		return err
	}
	seats, err := calls.Seats.Seats(connection.DomainId, record.CallId)
	if err != nil {
		return err
	}
	err = calls.seatIfNew(connection, record, seats)
	if err != nil {
		return err
	}
	return calls.answer(connection, message, record, FrameTypes{}.CallJoined())
}

func (calls Calls) Ring(connection Connection, message ClientMessage) error {
	record, _, err := calls.inCall(connection, message, SignalCheck{}.Ring(message))
	if err != nil {
		return err
	}
	frame := CallFrame{Type: FrameTypes{}.CallRinging(), CallId: record.CallId, FromPersonId: connection.MemberId, Invite: message.Invite}
	split, err := calls.toPeople(connection.DomainId, MemberSet{}.Sorted(message.ToPersonIds), frame)
	if err != nil {
		return err
	}
	return calls.pushed(connection.DomainId, split.Missed, calls.incoming())
}

func (calls Calls) EndRinging(connection Connection, message ClientMessage) error {
	record, seats, err := calls.inCall(connection, message, SignalCheck{}.RingEnd(message))
	if err != nil {
		return err
	}
	frame := CallFrame{Type: FrameTypes{}.CallRingEnded(), CallId: record.CallId}
	people := MemberSet{}.Sorted(append(slices.Clone(message.Missed), calls.Seats.People(seats)...))
	_, err = calls.toPeople(connection.DomainId, people, frame)
	if err != nil {
		return err
	}
	return calls.pushed(connection.DomainId, MemberSet{}.Sorted(message.Missed), calls.missed())
}

func (calls Calls) Leave(connection Connection, message ClientMessage) error {
	err := calls.device(connection, SignalCheck{}.Call(message))
	if err != nil {
		return err
	}
	record, err := calls.Book.Find(connection.DomainId, message.CallId)
	if err != nil {
		return err
	}
	seats, err := calls.Seats.Seats(connection.DomainId, record.CallId)
	if err != nil {
		return err
	}
	mine := calls.Seats.Of(seats, connection.DeviceId)
	err = calls.Seats.Remove(connection.DomainId, mine)
	if err != nil {
		return err
	}
	err = calls.endIfEmpty(connection.DomainId, record, len(seats)-len(mine))
	if err != nil {
		return err
	}
	return calls.Pushes.Send(connection.ConnectionId, CallFrame{Type: FrameTypes{}.CallLeft(), RequestId: message.RequestId, CallId: record.CallId})
}

func (calls Calls) device(connection Connection, checked error) error {
	if checked != nil {
		return checked
	}
	return SignalCheck{}.Device(connection)
}

func (calls Calls) member(connection Connection, checked error) error {
	err := calls.device(connection, checked)
	if err != nil {
		return err
	}
	return SignalCheck{}.Linked(connection)
}

func (calls Calls) inCall(connection Connection, message ClientMessage, checked error) (CallRecord, []SeatRecord, error) {
	err := calls.device(connection, checked)
	if err != nil {
		return CallRecord{}, nil, err
	}
	record, err := calls.Book.Find(connection.DomainId, message.CallId)
	if err != nil {
		return CallRecord{}, nil, err
	}
	seats, err := calls.Seats.Seats(connection.DomainId, record.CallId)
	if err != nil {
		return CallRecord{}, nil, err
	}
	return record, seats, calls.seated(seats, connection.DeviceId)
}

func (calls Calls) seated(seats []SeatRecord, deviceId string) error {
	if len(calls.Seats.Of(seats, deviceId)) == 0 {
		return MissingTarget{}.NotInCall()
	}
	return nil
}

func (calls Calls) signable(group bool) error {
	if !group {
		return nil
	}
	return calls.Signer.Ready()
}

func (calls Calls) seatIfNew(connection Connection, record CallRecord, seats []SeatRecord) error {
	if len(calls.Seats.Of(seats, connection.DeviceId)) > 0 {
		return nil
	}
	err := calls.room(seats, connection.MemberId)
	if err != nil {
		return err
	}
	err = calls.Seats.Add(connection.DomainId, calls.seat(record.CallId, connection))
	if err != nil {
		return err
	}
	return calls.Book.Touch(connection.DomainId, record, calls.now())
}

func (calls Calls) room(seats []SeatRecord, personId string) error {
	people := calls.Seats.People(seats)
	if !slices.Contains(people, personId) && len(people) >= (SignalLimits{}).MaximumCallPeople() {
		return CallRefusal{}.CallFull()
	}
	return nil
}

func (calls Calls) answer(connection Connection, message ClientMessage, record CallRecord, frameType string) error {
	token, err := calls.token(record, connection)
	if err != nil {
		return err
	}
	return calls.Pushes.Send(connection.ConnectionId, CallFrame{Type: frameType, RequestId: message.RequestId, CallId: record.CallId, JoinToken: token})
}

func (calls Calls) token(record CallRecord, connection Connection) (string, error) {
	if !record.Group {
		return "", nil
	}
	return calls.Signer.Token(record.CallId, connection.DeviceId, connection.DomainId)
}

func (calls Calls) toPeople(domainId string, people []string, frame CallFrame) (DeliverySplit, error) {
	found, err := calls.Connections.ForMembers(domainId, people)
	if err != nil {
		return DeliverySplit{}, err
	}
	return FrameDelivery{Pushes: calls.Pushes}.Split(ConnectionGroups{}.ByMember(found), people, frame)
}

func (calls Calls) pushed(domainId string, people []string, kind string) error {
	if len(people) == 0 {
		return nil
	}
	return calls.logged(domainId, kind, calls.Notify.Send(domainId, CallPush{ToPersonIds: people, Kind: kind}))
}

func (calls Calls) logged(domainId string, kind string, failure error) error {
	if failure != nil {
		calls.Log.PushFailed(domainId, kind, failure)
	}
	return nil
}

func (calls Calls) endIfEmpty(domainId string, record CallRecord, remaining int) error {
	if remaining > 0 {
		return calls.Book.Touch(domainId, record, calls.now())
	}
	return calls.Book.Delete(domainId, record)
}

func (calls Calls) seat(callId string, connection Connection) SeatRecord {
	return SeatRecord{CallId: callId, DeviceId: connection.DeviceId, PersonId: connection.MemberId, JoinedAt: calls.now()}
}

func (calls Calls) newId() string {
	return "call_" + rand.Text()
}

func (calls Calls) now() string {
	return calls.Now().UTC().Format(time.RFC3339)
}

func (Calls) incoming() string {
	return "incoming"
}

func (Calls) missed() string {
	return "missed"
}
