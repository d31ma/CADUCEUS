import "strings"

// @TestSetup
type ChannelTestSetup struct{}

func (ChannelTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["channels"] = ChannelTestSetup{}
	config.Http["identity"] = ChannelTestSetup{}
	return config, nil
}

func (ChannelTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if strings.HasPrefix(request.Path, "/api/v1/identity") {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	if request.Method == "GET" {
		return ChannelTestSetup{}.channels(), nil
	}
	body, _ := request.Body.(map[string]any)
	return ChannelTestSetup{}.updated(strings.TrimPrefix(request.Path, "/api/v1/channels/"), body)
}

func (ChannelTestSetup) updated(channelId string, body map[string]any) (BehaviorValue, error) {
	if channelId == "gone-id" {
		return nil, &BehaviorError{Code: "storage", Message: "not found", Status: 404}
	}
	if channelId == "locked-id" {
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	}
	if body["emailAddress"] == "bo@example.com" {
		return nil, &BehaviorError{Code: "storage", Message: "taken", Status: 409}
	}
	if body["emailAddress"] == "down@example.com" {
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return ChannelTestSetup{}.changed(ChannelTestSetup{}.channel(channelId), body), nil
}

func (ChannelTestSetup) changed(channel map[string]any, body map[string]any) map[string]any {
	if address, given := body["emailAddress"]; given && address != nil {
		channel["emailAddress"] = address
	}
	if archived, given := body["archived"]; given && archived != nil {
		channel["archived"] = archived
	}
	return channel
}

func (ChannelTestSetup) channel(channelId string) map[string]any {
	channels := ChannelTestSetup{}.channels()
	for _, channel := range channels {
		found := channel.(map[string]any)
		if found["channelId"] == channelId {
			return found
		}
	}
	return map[string]any{}
}

func (ChannelTestSetup) channels() []any {
	return []any{
		ChannelTestSetup{}.row("general-id", "general", "Company news", "public", "", false),
		ChannelTestSetup{}.row("support-id", "support", "Customer questions", "public", "support@example.com", false),
		ChannelTestSetup{}.row("design-id", "design", "Design reviews", "private", "", false),
		ChannelTestSetup{}.row("launch-id", "launch-q3", "Launch plans", "public", "", true),
		ChannelTestSetup{}.row("gone-id", "old-news", "Old news", "public", "", false),
		ChannelTestSetup{}.row("locked-id", "board", "Board notes", "private", "board@example.com", false),
	}
}

func (ChannelTestSetup) row(channelId string, name string, topic string, visibility string, address string, archived bool) map[string]any {
	return map[string]any{
		"channelId":      channelId,
		"name":           name,
		"topic":          topic,
		"visibility":     visibility,
		"emailAddress":   address,
		"archived":       archived,
		"conversationId": channelId + "-conversation",
	}
}
