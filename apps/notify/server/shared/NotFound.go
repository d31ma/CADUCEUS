type NotFound struct {
	ErrorCase
}

func (NotFound) InstallWideRoute() *NotFound {
	return &NotFound{ErrorCase{4502, "not_found", "", "The install-wide route was not found."}}
}

func (NotFound) Domain() *NotFound {
	return &NotFound{ErrorCase{4504, "not_found", "", "The domain was not found."}}
}

func (NotFound) Notification() *NotFound {
	return &NotFound{ErrorCase{4510, "not_found", "", "The notification was not found."}}
}

func (NotFound) Message() *NotFound {
	return &NotFound{ErrorCase{4511, "not_found", "", "The message was not found."}}
}

func (NotFound) OutboxDocument() *NotFound {
	return &NotFound{ErrorCase{4520, "not_found", "", "The outbox document was not found."}}
}

func (NotFound) OutboxCollection() *NotFound {
	return &NotFound{ErrorCase{4521, "not_found", "", "The outbox collection was not found."}}
}

func (NotFound) DeadLetter() *NotFound {
	return &NotFound{ErrorCase{4522, "not_found", "", "The dead letter was not found."}}
}
