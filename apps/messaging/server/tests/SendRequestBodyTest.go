import (
	"encoding/json"
	"reflect"
	"strings"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type SendRequestBodyTest struct {
	behaviors *BehaviorContext
}

func (test *SendRequestBodyTest) decoded(world *MessagingTestSetup, body string) SendRequest {
	request := SendRequest{}
	world.Must(json.Unmarshal([]byte(body), &request))
	return request
}

func (test *SendRequestBodyTest) omitsWhenEmpty(model any, field string) bool {
	tag, _ := reflect.TypeOf(model).FieldByName(field)
	return strings.HasSuffix(tag.Tag.Get("json"), ",omitempty")
}

func (test *SendRequestBodyTest) TestAShortSendBodyIsAccepted() {
	world := MessagingTestSetup{}.World()
	body := `{"clientMessageId":"c1","body":"hi","recipients":[{"address":"` + world.Address("bo") + `","role":"to","delivery":"chat","expectedKind":"member"}]}`
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), test.decoded(world, body))
	Assert.IsTrue(err == nil, "a send without conversationId, attachmentIds and alsoToConversation must work")
	Assert.Equals("hi", result.Message.Body)
	Assert.Equals([]AttachmentRef{}, result.Message.Attachments)
}

func (test *SendRequestBodyTest) TestAReplyBodyNeedsNoRecipients() {
	world := MessagingTestSetup{}.World()
	first := world.ChatTo("bo", "c1", "first")
	body := `{"clientMessageId":"c2","conversationId":"` + first.Message.ConversationId + `","body":"second"}`
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), test.decoded(world, body))
	Assert.IsTrue(err == nil, "a reply without recipients must work")
	Assert.Equals(first.Message.ConversationId, result.Message.ConversationId)
}

func (test *SendRequestBodyTest) TestTheCoreStillJudgesMissingValues() {
	world := MessagingTestSetup{}.World()
	blank := `{"clientMessageId":"c1","recipients":[{"address":"` + world.Address("bo") + `","role":"to","delivery":"chat","expectedKind":"member"}]}`
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), test.decoded(world, blank))
	Assert.Equals("body", world.InvalidField(err))
	noSubject := `{"clientMessageId":"c2","body":"hi","email":{},"recipients":[{"address":"carol@partner.org","role":"to","delivery":"email","expectedKind":"external"}]}`
	_, err = world.Messages.PostMessage(world.MemberBearer("ana"), test.decoded(world, noSubject))
	Assert.Equals("subject", world.InvalidField(err))
}

func (test *SendRequestBodyTest) TestOnlyTheClientMessageIdIsRequired() {
	Assert.IsTrue(!test.omitsWhenEmpty(SendRequest{}, "ClientMessageId"), "clientMessageId stays required")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "ConversationId"), "conversationId is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "ThreadRootId"), "threadRootId is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "AlsoToConversation"), "alsoToConversation is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "Body"), "body is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "AttachmentIds"), "attachmentIds is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "Recipients"), "recipients is optional")
	Assert.IsTrue(test.omitsWhenEmpty(SendRequest{}, "Email"), "email is optional")
	Assert.IsTrue(test.omitsWhenEmpty(EmailOptions{}, "Subject"), "subject is optional")
	Assert.IsTrue(test.omitsWhenEmpty(EmailOptions{}, "FromAddress"), "fromAddress is optional")
	Assert.IsTrue(test.omitsWhenEmpty(EmailOptions{}, "SendToExternal"), "sendToExternal is optional")
}
