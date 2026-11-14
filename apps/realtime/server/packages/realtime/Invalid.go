package realtime

type Invalid struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (Invalid) ConnectionIdMissing() Invalid {
	return Invalid{Field: "connectionId", Reason: "a connection id is required", ErrorNumber: 5001}
}

func (Invalid) UnknownKind() Invalid {
	return Invalid{Field: "kind", Reason: "the kind is socket or stream", ErrorNumber: 5002}
}

func (Invalid) UnknownMessageType() Invalid {
	return Invalid{Field: "type", Reason: "unknown message type", ErrorNumber: 5003}
}

func (Invalid) ConversationIdMissing() Invalid {
	return Invalid{Field: "conversationId", Reason: "a conversation id is required", ErrorNumber: 5004}
}

func (Invalid) WrongMemberCount() Invalid {
	return Invalid{Field: "memberIds", Reason: "must name 1 to 100 members", ErrorNumber: 5005}
}

func (Invalid) NowNotATime() Invalid {
	return Invalid{Field: "now", Reason: "must be an RFC 3339 time", ErrorNumber: 5006}
}

func (Invalid) UnknownState() Invalid {
	return Invalid{Field: "state", Reason: "must be active, idle or dnd", ErrorNumber: 5007}
}

func (Invalid) EndNotInFuture() Invalid {
	return Invalid{Field: "doNotDisturbUntil", Reason: "must be an RFC 3339 time in the future", ErrorNumber: 5008}
}

func (Invalid) DeviceIdInvalid() Invalid {
	return Invalid{Field: "deviceId", Reason: "must be 1 to 64 letters, digits, - or _", ErrorNumber: 5026}
}

func (failure Invalid) Error() string {
	return "invalid " + failure.Field + ": " + failure.Reason
}

func (failure Invalid) Number() int {
	return failure.ErrorNumber
}
