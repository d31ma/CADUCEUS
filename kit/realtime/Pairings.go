package realtime

type Pairings struct {
	Book        PairingBook
	Connections ConnectionBook
	Pushes      ConnectionGateway
}

func (pairings Pairings) Open(connection Connection, message ClientMessage) error {
	err := pairings.checked(connection, message)
	if err != nil {
		return err
	}
	record, found, err := pairings.Book.Find(connection.DomainId, message.PairingId)
	if err != nil {
		return err
	}
	err = pairings.free(record, found, connection)
	if err != nil {
		return err
	}
	opened := PairingRecord{ID: pairings.reused(record, found), PairingId: message.PairingId, PersonId: connection.MemberId, OpenerDeviceId: connection.DeviceId}
	err = pairings.Book.Open(connection.DomainId, opened)
	if err != nil {
		return err
	}
	answer := PairingFrame{Type: FrameTypes{}.PairingOpened(), RequestId: message.RequestId, PairingId: message.PairingId}
	return pairings.Pushes.Send(connection.ConnectionId, answer)
}

func (pairings Pairings) Join(connection Connection, message ClientMessage) error {
	err := pairings.checked(connection, message)
	if err != nil {
		return err
	}
	record, found, err := pairings.Book.Find(connection.DomainId, message.PairingId)
	if err != nil {
		return err
	}
	err = pairings.joinable(record, found, connection)
	if err != nil {
		return err
	}
	err = pairings.Book.Join(connection.DomainId, record, connection.DeviceId)
	if err != nil {
		return err
	}
	return pairings.ready(connection, message, record)
}

func (pairings Pairings) checked(connection Connection, message ClientMessage) error {
	err := SignalCheck{}.Pairing(message)
	if err != nil {
		return err
	}
	return SignalCheck{}.Device(connection)
}

func (pairings Pairings) free(record PairingRecord, found bool, connection Connection) error {
	held := found && pairings.Book.Current(record)
	if held && (record.PersonId != connection.MemberId || record.OpenerDeviceId != connection.DeviceId) {
		return MissingTarget{}.PairingNotFound()
	}
	return nil
}

func (pairings Pairings) reused(record PairingRecord, found bool) string {
	if found {
		return record.ID
	}
	return ""
}

func (pairings Pairings) joinable(record PairingRecord, found bool, connection Connection) error {
	usable := found && pairings.Book.Current(record) && record.PersonId == connection.MemberId
	mine := record.OpenerDeviceId != connection.DeviceId && (record.JoinerDeviceId == "" || record.JoinerDeviceId == connection.DeviceId)
	if !usable || !mine {
		return MissingTarget{}.PairingNotFound()
	}
	return nil
}

func (pairings Pairings) ready(connection Connection, message ClientMessage, record PairingRecord) error {
	found, err := pairings.Connections.InDomain(connection.DomainId)
	if err != nil {
		return err
	}
	groups := ConnectionGroups{}.ByDevice(found)
	toOpener := PairingFrame{Type: FrameTypes{}.PairingReady(), PairingId: record.PairingId, PeerDeviceId: connection.DeviceId}
	_, err = FrameDelivery{Pushes: pairings.Pushes}.Reached(groups[record.OpenerDeviceId], toOpener)
	if err != nil {
		return err
	}
	toJoiner := PairingFrame{Type: FrameTypes{}.PairingReady(), RequestId: message.RequestId, PairingId: record.PairingId, PeerDeviceId: record.OpenerDeviceId}
	return pairings.Pushes.Send(connection.ConnectionId, toJoiner)
}
