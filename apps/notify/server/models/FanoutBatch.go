import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

type FanoutBatch struct {
	DomainId   string                          `json:"domainId"`
	Message    events.MessageCreated           `json:"message"`
	Recipients []notify.Recipient              `json:"recipients"`
	Settings   map[string]MemberSettingsRecord `json:"settings"`
}
