// @Test(MailboxTestSetup)
type ThreadingTest struct {
	behaviors *BehaviorContext
}

// @ValueSource(["abc@example.com", "<abc@example.com>", " < abc@example.com > "])
func (test *ThreadingTest) TestNormalizesAMessageId(value string) {
	Assert.Equals("<abc@example.com>", Threading{}.NormalizeMessageId(value))
}

// @ValueSource(["", "abc", "@example.com", "abc@", "a b@example.com", "<a@b> <c@d>", "a\r\nb@c"])
func (test *ThreadingTest) TestRejectsAnInvalidMessageId(value string) {
	Assert.Equals("", Threading{}.NormalizeMessageId(value))
}

func (test *ThreadingTest) TestParsesAReferencesHeader() {
	threading := Threading{}
	Assert.Equals([]string{"<a@x>", "<b@x>"}, threading.ParseMessageIdList("<a@x> <b@x>\r\n <a@x>"))
	Assert.Equals([]string{"<a@x>", "<b@x>"}, threading.ParseMessageIdList("a@x, b@x"))
	Assert.Equals([]string{"<a@x>"}, threading.ParseMessageIdList([]any{"a@x", 7.0}))
}

func (test *ThreadingTest) TestRootsAReplyAtItsFirstReference() {
	threading := Threading{}
	metadata := threading.ThreadMetadataFromHeaders(map[string]any{"message-id": "<c@x>", "in-reply-to": "<b@x>", "references": "<a@x> <b@x>"}, "f", "Example.com")
	Assert.Equals("<c@x>", metadata["rfcMessageId"])
	Assert.Equals("<b@x>", metadata["inReplyTo"])
	Assert.Equals([]string{"<a@x>", "<b@x>"}, metadata["references"])
	Assert.Equals(threading.DeriveThreadId("example.com", "<a@x>"), metadata["threadId"])
}

func (test *ThreadingTest) TestCreatesAnIdForAComposedMessage() {
	threading := Threading{}
	metadata := threading.ThreadMetadataFromOutbound(map[string]any{}, "Draft 1", "example.com")
	id, _ := metadata["rfcMessageId"].(string)
	Assert.IsTrue(threading.NormalizeMessageId(id) == id, id)
	Assert.Equals(threading.DeriveThreadId("example.com", id), metadata["threadId"])
	_, hasReferences := metadata["references"]
	Assert.IsTrue(!hasReferences)
}

func (test *ThreadingTest) TestShapesAMessageId() {
	id := Threading{}.CreateMessageId("Mail.Example.com!", "Draft 1")
	Assert.Equals("<draft1.", id[:8])
	Assert.Equals("@mail.example.com>", id[len(id)-18:])
}
