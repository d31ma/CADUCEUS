type InvalidRequest struct {
	ErrorCase
}

func (InvalidRequest) BadEnvelope(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4505, "invalid_request", "envelope", reason}}
}

func (InvalidRequest) Kit(field string, reason string, number int) *InvalidRequest {
	return &InvalidRequest{ErrorCase{number, "invalid_request", field, reason}}
}

func (InvalidRequest) UnknownEventType(eventType string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4506, "invalid_request", "type", "notify does not handle events of type " + eventType}}
}

func (InvalidRequest) BadTime() *InvalidRequest {
	return &InvalidRequest{ErrorCase{4507, "invalid_request", "time", "the time is not an RFC 3339 time"}}
}

func (InvalidRequest) BadLimit(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4508, "invalid_request", "limit", reason}}
}

func (InvalidRequest) BadCursor(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4509, "invalid_request", "cursor", reason}}
}

func (InvalidRequest) BadSummaryDelay(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4512, "invalid_request", "emailSummaryAfterMinutes", reason}}
}

func (InvalidRequest) BadMutedList(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4513, "invalid_request", "mutedConversationIds", reason}}
}

func (InvalidRequest) BadEndpoint(reason string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{4514, "invalid_request", "endpoint", reason}}
}

func (InvalidRequest) MissingKeys() *InvalidRequest {
	return &InvalidRequest{ErrorCase{4515, "invalid_request", "p256dh", "the subscription keys p256dh and auth are required"}}
}

func (InvalidRequest) BadCallKind() *InvalidRequest {
	return &InvalidRequest{ErrorCase{4523, "invalid_request", "kind", "the kind must be incoming or missed"}}
}

func (InvalidRequest) BadPersonList() *InvalidRequest {
	return &InvalidRequest{ErrorCase{4524, "invalid_request", "toPersonIds", "toPersonIds needs 1 to 49 ids of 1 to 64 characters"}}
}
