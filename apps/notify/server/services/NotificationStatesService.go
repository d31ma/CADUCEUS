// @Service
type NotificationStatesService struct{}

func (states *NotificationStatesService) PushPending() string {
	return "pending"
}

func (states *NotificationStatesService) PushSent() string {
	return "sent"
}

func (states *NotificationStatesService) PushSkipped() string {
	return "skipped"
}

func (states *NotificationStatesService) PushFailed() string {
	return "failed"
}

func (states *NotificationStatesService) SummaryWaiting() string {
	return "waiting"
}

func (states *NotificationStatesService) SummaryNone() string {
	return states.none()
}

func (states *NotificationStatesService) none() string {
	return "none"
}
