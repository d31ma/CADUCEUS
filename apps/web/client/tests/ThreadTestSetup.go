import (
	"encoding/json"
	"errors"
	"fmt"
)

// @TestSetup
type ThreadTestSetup struct{}

func (setup ThreadTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["identity"] = setup
	configuration.Http["IdentityClient"] = setup
	configuration.Http["messaging"] = setup
	configuration.Http["thread"] = setup
	configuration.Http["ThreadClient"] = setup
	return configuration, nil
}

func (setup ThreadTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Method+" "+request.Path == "POST /api/v1/identity/session/refresh" {
		return setup.refresh(request)
	}
	if !setup.known(fmt.Sprint(request.Headers["Authorization"])) {
		return nil, errors.New("unauthenticated")
	}
	switch request.Method + " " + request.Path {
	case "GET /api/v1/messaging/conversations/conv-1/messages":
		return setup.page(fmt.Sprint(request.Query["threadRootId"]), fmt.Sprint(request.Query["cursor"]))
	case "GET /api/v1/messaging/messages/root-1":
		return setup.root(), nil
	case "POST /api/v1/messaging/messages":
		return setup.send(request)
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup ThreadTestSetup) known(authorization string) bool {
	return authorization == "Bearer access-1" || authorization == "Bearer access-2"
}

func (setup ThreadTestSetup) refresh(request *BehaviorHttpRequest) (BehaviorValue, error) {
	text, _ := json.Marshal(request.Body)
	body := SessionRefreshRequest{}
	_ = json.Unmarshal(text, &body)
	if body.SessionKey != "key-1" {
		return nil, errors.New("unauthenticated")
	}
	me := Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}}
	return Session{Me: me, AccessToken: "access-2", ExpiresIn: 3600, SessionKey: "key-2"}, nil
}

func (setup ThreadTestSetup) page(rootId string, cursor string) (BehaviorValue, error) {
	switch rootId + " " + cursor {
	case "root-1 <nil>", "root-1 ":
		items := []Message{setup.reply("reply-3", "Bo Lee", "09:31"), setup.reply("reply-2", "Bo Lee", "09:20")}
		return MessagePage{Items: items, NextCursor: "c-2"}, nil
	case "root-1 c-2":
		return MessagePage{Items: []Message{setup.reply("reply-1", "Cy Diaz", "09:15")}, NextCursor: ""}, nil
	case "root-2 <nil>", "root-2 ":
		return MessagePage{Items: []Message{setup.reply("reply-a", "Bo Lee", "10:00")}, NextCursor: ""}, nil
	}
	return nil, errors.New("unavailable")
}

func (setup ThreadTestSetup) root() Message {
	author := Author{Kind: "member", MemberId: "m-1", Address: "ana@example.com", DisplayName: "Ana Ruiz"}
	return Message{MessageId: "root-1", ConversationId: "conv-1", Author: author, Body: "Build 42 is green.", ReplyCount: 3, CreatedAt: "2026-11-10T09:12:00Z"}
}

func (setup ThreadTestSetup) reply(messageId string, name string, clock string) Message {
	author := Author{Kind: "member", Address: "someone@example.com", DisplayName: name}
	return Message{MessageId: messageId, ConversationId: "conv-1", ThreadRootId: "root-1", Author: author, Body: "Text of " + messageId, CreatedAt: "2026-11-10T" + clock + ":00Z"}
}

func (setup ThreadTestSetup) send(request *BehaviorHttpRequest) (BehaviorValue, error) {
	text, _ := json.Marshal(request.Body)
	body := ThreadReplyRequest{}
	_ = json.Unmarshal(text, &body)
	if body.Body == "offline" {
		return nil, errors.New("offline")
	}
	author := Author{Kind: "member", Address: "ana@example.com", DisplayName: "Ana Ruiz"}
	message := Message{MessageId: "msg-" + body.ClientMessageId, ConversationId: body.ConversationId, ThreadRootId: body.ThreadRootId, AlsoToConversation: body.AlsoToConversation, Author: author, Body: body.Body, CreatedAt: "2026-11-10T09:40:00Z"}
	return ThreadSendResult{Message: message, Deliveries: []DeliveryStatus{}}, nil
}
