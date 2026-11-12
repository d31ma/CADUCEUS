package events

import "encoding/json"

type ReplaceKeys struct{}

func (keys ReplaceKeys) Of(envelope Envelope) (EventKey, error) {
	return keys.keyOf(envelope.Payload, keys.replaceRules()[envelope.Type])
}

func (keys ReplaceKeys) Removes(envelope Envelope) (EventKey, error) {
	return keys.keyOf(envelope.Payload, keys.removeRules()[envelope.Type])
}

func (ReplaceKeys) keyOf(payload json.RawMessage, rule KeyRule) (EventKey, error) {
	if rule.Field == "" {
		return EventKey{}, nil
	}
	fields := KeyFields{}
	err := json.Unmarshal(payload, &fields)
	return fields.Key(rule), err
}

func (ReplaceKeys) replaceRules() map[string]KeyRule {
	types := Types{}
	fields := KeyFields{}
	return map[string]KeyRule{
		types.MessageEdited():   {Type: types.MessageEdited(), Field: fields.MessageIdField()},
		types.ReactionChanged(): {Type: types.ReactionChanged(), Field: fields.MessageIdField()},
		types.PresenceChanged(): {Type: types.PresenceChanged(), Field: fields.MemberIdField()},
	}
}

func (ReplaceKeys) removeRules() map[string]KeyRule {
	types := Types{}
	return map[string]KeyRule{
		types.MessageDeleted(): {Type: types.MessageEdited(), Field: KeyFields{}.MessageIdField()},
	}
}
