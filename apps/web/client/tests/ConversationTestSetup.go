import "errors"

// @TestSetup
type ConversationTestSetup struct{}

func (setup ConversationTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["messaging"] = setup
	configuration.Http["MessagingClient"] = setup
	configuration.Http["channels"] = setup
	configuration.Http["ChannelsClient"] = setup
	return configuration, nil
}

func (setup ConversationTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if setup.authorizationOf(request) != "Bearer access-1" {
		return nil, errors.New("unauthenticated")
	}
	switch request.Method + " " + request.Path {
	case "GET /api/v1/channels":
		return setup.Channels(), nil
	case "GET /api/v1/messaging/conversations":
		return ConversationPage{Items: setup.Summaries(), NextCursor: ""}, nil
	case "GET /api/v1/messaging/conversations/cnv_general/messages":
		return setup.generalPage(setup.cursorOf(request)), nil
	case "GET /api/v1/messaging/conversations/cnv_carol/messages":
		return MessagePage{Items: []Message{setup.EmailMessage()}, NextCursor: ""}, nil
	case "GET /api/v1/messaging/conversations/cnv_empty/messages":
		return MessagePage{Items: []Message{}, NextCursor: ""}, nil
	case "PATCH /api/v1/messaging/messages/m-2":
		return setup.edited(), nil
	case "DELETE /api/v1/messaging/messages/m-2",
		"PUT /api/v1/messaging/messages/m-3/reactions/thumbs_up",
		"DELETE /api/v1/messaging/messages/m-3/reactions/thumbs_up",
		"PUT /api/v1/messaging/conversations/cnv_general/read",
		"PUT /api/v1/messaging/conversations/cnv_carol/read",
		"PUT /api/v1/messaging/conversations/cnv_empty/read":
		return nil, nil
	case "GET /api/v1/messaging/messages/m-9/original":
		return MessageDownload{Url: "https://files.example.com/m-9.eml", ExpiresAt: "2026-11-10T15:00:00Z"}, nil
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup ConversationTestSetup) Channels() []Channel {
	return []Channel{
		{ChannelId: "ch-general", Name: "general", Visibility: "public", EmailAddress: "general@example.com", ConversationId: "cnv_general"},
		{ChannelId: "ch-secret", Name: "secret", Visibility: "private", Archived: true, ConversationId: "cnv_secret"},
	}
}

func (setup ConversationTestSetup) Summaries() []ConversationSummary {
	return []ConversationSummary{
		{ConversationId: "cnv_secret", Kind: "channel", Title: "secret", LastMessageAt: "2026-11-09T10:00:00Z"},
		{ConversationId: "cnv_general", Kind: "channel", Title: "general", UnreadCount: 2, MentionCount: 1, LastMessageAt: "2026-11-10T14:07:00Z"},
		{ConversationId: "cnv_bo", Kind: "direct", Title: "Bo Lee", UnreadCount: 1, LastMessageAt: "2026-11-10T09:00:00Z"},
		{ConversationId: "cnv_team", Kind: "direct", Title: "Bo Lee, Dee Park", LastMessageAt: "2026-11-10T12:00:00Z"},
		{ConversationId: "cnv_carol", Kind: "direct", Title: "Carol Diaz", HasExternal: true, LastMessageAt: "2026-11-10T13:00:00Z"},
	}
}

func (setup ConversationTestSetup) GeneralMessages() []Message {
	return []Message{
		setup.chat("m-3", "bo", "Bo Lee", "2026-11-10T14:07:00Z", "Thanks @ana"),
		setup.mine("m-2", "2026-11-10T14:05:00Z", "Lunch at noon?"),
		setup.chat("m-1", "bo", "Bo Lee", "2026-11-09T09:00:00Z", "Hello team"),
	}
}

func (setup ConversationTestSetup) OlderMessages() []Message {
	return []Message{setup.chat("m-0", "dee", "Dee Park", "2026-11-08T09:00:00Z", "Welcome")}
}

func (setup ConversationTestSetup) EmailMessage() Message {
	message := setup.chat("m-9", "carol", "Carol Diaz", "2026-11-10T13:00:00Z", "Hi Ana")
	message.Author = Author{Kind: "external", Address: "carol@outside.test", DisplayName: "Carol Diaz"}
	message.BodyFormat = "email"
	message.Truncated = true
	message.Email = &MessageEmailSummary{Subject: "Quote", From: "carol@outside.test", To: []string{"ana@example.com"}, Cc: []string{}, AuthResults: MessageAuthResults{SenderPolicy: "pass", SignatureCheck: "pass", DomainPolicy: "pass"}}
	message.Blocks = append(message.Blocks,
		Block{Kind: "image", Image: &ImageRef{Alt: "Logo", RemoteUrl: "https://outside.test/logo.png"}},
		Block{Kind: "quote", Level: 1, Spans: []Span{{Text: "Earlier message"}}},
	)
	return message
}

func (setup ConversationTestSetup) chat(messageId string, memberId string, name string, createdAt string, text string) Message {
	return Message{
		MessageId:      messageId,
		ConversationId: "cnv_general",
		Author:         Author{Kind: "member", MemberId: memberId, Address: memberId + "@example.com", DisplayName: name},
		Body:           text,
		BodyFormat:     "markdown",
		Blocks:         []Block{{Kind: "paragraph", Spans: []Span{{Text: text}}}},
		Attachments:    []AttachmentRef{},
		Reactions:      map[string]int{"thumbs_up": 1},
		MyReactions:    []string{},
		CreatedAt:      createdAt,
		Deliveries:     []DeliveryStatus{},
	}
}

func (setup ConversationTestSetup) mine(messageId string, createdAt string, text string) Message {
	message := setup.chat(messageId, "ana", "Ana Ruiz", createdAt, text)
	message.Deliveries = []DeliveryStatus{
		{Recipient: "bo@example.com", Channel: "chat", Status: "delivered"},
		{Recipient: "carol@outside.test", Channel: "email", Status: "sent"},
	}
	return message
}

func (setup ConversationTestSetup) edited() Message {
	message := setup.mine("m-2", "2026-11-10T14:05:00Z", "Lunch at one?")
	message.EditedAt = "2026-11-10T14:06:00Z"
	return message
}

func (setup ConversationTestSetup) generalPage(cursor string) MessagePage {
	if cursor == "older" {
		return MessagePage{Items: setup.OlderMessages(), NextCursor: ""}
	}
	return MessagePage{Items: setup.GeneralMessages(), NextCursor: "older"}
}

func (setup ConversationTestSetup) authorizationOf(request *BehaviorHttpRequest) string {
	text, _ := request.Headers["Authorization"].(string)
	return text
}

func (setup ConversationTestSetup) cursorOf(request *BehaviorHttpRequest) string {
	text, _ := request.Query["cursor"].(string)
	return text
}
