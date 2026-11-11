// @Test(MessagingTestSetup, {"partition":"tests"})
type MessageSourceControllerTest struct {
	behaviors *BehaviorContext
}

func (test *MessageSourceControllerTest) TestGetSourceAnswersTheBodyToAServiceOfTheDomain() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "the release is **done**")
	source, err := world.Sources.GetSource(world.ServiceBearer(world.Home), sent.Message.MessageId)
	Assert.IsTrue(err == nil, "reading the source failed")
	Assert.Equals(MessageSource{MessageId: sent.Message.MessageId, DomainId: world.Domain, Body: "the release is **done**", AttachmentIds: []string{}}, source)
}

func (test *MessageSourceControllerTest) TestGetSourceRefusesAMemberToken() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	_, err := world.Sources.GetSource(world.MemberBearer("bo"), sent.Message.MessageId)
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *MessageSourceControllerTest) TestGetSourceForAnotherDomainIsNotFound() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	_, err := world.Sources.GetSource(world.ServiceBearer(world.Away), sent.Message.MessageId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}
