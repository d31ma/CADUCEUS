package channels

import "strings"

type WireIds struct{}

func (ids WireIds) ChannelId(storedId string) string {
	return ids.channelPrefix() + storedId
}

func (ids WireIds) ConversationId(storedId string) string {
	return ids.conversationPrefix() + storedId
}

func (ids WireIds) StoredId(channelId string) (string, error) {
	storedId, found := strings.CutPrefix(channelId, ids.channelPrefix())
	return storedId, ids.refusal(found, storedId)
}

func (WireIds) refusal(found bool, storedId string) error {
	if found && storedId != "" {
		return nil
	}
	return &NotFound{Message: "no such channel"}
}

func (WireIds) channelPrefix() string {
	return "ch_"
}

func (WireIds) conversationPrefix() string {
	return "cnv_"
}
