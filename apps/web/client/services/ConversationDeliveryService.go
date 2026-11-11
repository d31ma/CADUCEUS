import (
	"cmp"
	"strconv"
)

// @Service
type ConversationDeliveryService struct{}

func (service *ConversationDeliveryService) Lines(statuses []DeliveryStatus) []DeliveryLine {
	return service.linesOf(statuses)
}

func (service *ConversationDeliveryService) Summary(lines []DeliveryLine) DeliveryLine {
	worst := service.worst(lines)
	worst.StatusLabel = cmp.Or(service.problemLabel(worst), service.countedLabel(worst, len(lines)))
	return worst
}

func (service *ConversationDeliveryService) linesOf(statuses []DeliveryStatus) []DeliveryLine {
	lines := []DeliveryLine{}
	for _, status := range statuses {
		lines = append(lines, service.line(status))
	}
	return lines
}

func (service *ConversationDeliveryService) line(status DeliveryStatus) DeliveryLine {
	state := service.states()[status.Channel+":"+status.Status]
	return DeliveryLine{
		Recipient:    status.Recipient,
		Channel:      status.Channel,
		ChannelLabel: service.channelLabels()[status.Channel],
		Status:       status.Status,
		StatusLabel:  cmp.Or(state.StatusLabel, "Unknown"),
		Tone:         cmp.Or(state.Tone, service.infoTone()),
		Connector:    cmp.Or(state.Connector, service.forWord()),
		Detail:       status.Detail,
	}
}

func (service *ConversationDeliveryService) worst(lines []DeliveryLine) DeliveryLine {
	worst := DeliveryLine{}
	for _, line := range lines {
		worst = service.worse(worst, line)
	}
	return worst
}

func (service *ConversationDeliveryService) worse(current DeliveryLine, candidate DeliveryLine) DeliveryLine {
	if service.ranks()[candidate.Tone] > service.ranks()[current.Tone] {
		return candidate
	}
	return current
}

func (service *ConversationDeliveryService) problemLabel(worst DeliveryLine) string {
	if service.ranks()[worst.Tone] < service.ranks()[service.warningTone()] {
		return ""
	}
	return worst.StatusLabel + service.forWord() + worst.Recipient
}

func (service *ConversationDeliveryService) forWord() string {
	return " for "
}

func (service *ConversationDeliveryService) toWord() string {
	return " to "
}

func (service *ConversationDeliveryService) byWord() string {
	return " by "
}

func (service *ConversationDeliveryService) countedLabel(worst DeliveryLine, count int) string {
	if count <= service.onePerson() {
		return worst.StatusLabel
	}
	return worst.StatusLabel + worst.Connector + strconv.Itoa(count) + " people"
}

func (service *ConversationDeliveryService) onePerson() int {
	return 1
}

func (service *ConversationDeliveryService) states() map[string]DeliveryLine {
	return map[string]DeliveryLine{
		"chat:delivered": {StatusLabel: "Delivered", Tone: service.okTone(), Connector: service.toWord()},
		"chat:read":      {StatusLabel: "Seen", Tone: service.okTone(), Connector: service.byWord()},
		"email:queued":   {StatusLabel: "Queued", Tone: service.infoTone(), Connector: service.forWord()},
		"email:sent":     {StatusLabel: "Sent", Tone: service.okTone(), Connector: service.toWord()},
		"email:deferred": {StatusLabel: "Retrying", Tone: service.warningTone()},
		"email:bounced":  {StatusLabel: "Bounced", Tone: service.dangerTone()},
		"email:failed":   {StatusLabel: "Failed", Tone: service.dangerTone()},
	}
}

func (service *ConversationDeliveryService) channelLabels() map[string]string {
	return map[string]string{"chat": "Chat", "email": "Email"}
}

func (service *ConversationDeliveryService) ranks() map[string]int {
	return map[string]int{
		service.okTone():      1,
		service.infoTone():    2,
		service.warningTone(): 3,
		service.dangerTone():  4,
	}
}

func (service *ConversationDeliveryService) okTone() string {
	return "ok"
}

func (service *ConversationDeliveryService) infoTone() string {
	return "info"
}

func (service *ConversationDeliveryService) warningTone() string {
	return "warning"
}

func (service *ConversationDeliveryService) dangerTone() string {
	return "danger"
}
