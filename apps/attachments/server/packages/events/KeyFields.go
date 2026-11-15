package events

type KeyFields struct {
	MessageId string `json:"messageId"`
	MemberId  string `json:"memberId"`
}

func (KeyFields) MessageIdField() string { return "messageId" }
func (KeyFields) MemberIdField() string  { return "memberId" }

func (fields KeyFields) Key(rule KeyRule) EventKey {
	return fields.keyed(rule.Type, fields.byName()[rule.Field])
}

func (fields KeyFields) byName() map[string]string {
	return map[string]string{
		fields.MessageIdField(): fields.MessageId,
		fields.MemberIdField():  fields.MemberId,
	}
}

func (KeyFields) keyed(eventType string, value string) EventKey {
	if value == "" {
		return EventKey{}
	}
	return EventKey{Type: eventType, Key: value}
}
