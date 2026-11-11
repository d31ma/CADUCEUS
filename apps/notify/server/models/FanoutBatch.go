import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/notifications"
)

type FanoutBatch struct {
	DomainId   string                          `json:"domainId"`
	Message    events.MessageCreated           `json:"message"`
	Recipients []notifications.Recipient       `json:"recipients"`
	Settings   map[string]MemberSettingsRecord `json:"settings"`
}
