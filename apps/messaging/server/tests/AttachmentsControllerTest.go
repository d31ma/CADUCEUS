// @Test(MessagingTestSetup, {"partition":"tests"})
type AttachmentsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentsControllerTest) TestGetAccessAllowsAReaderOfTheMessage() {
	world := MessagingTestSetup{}.World()
	world.WithAttachment("bo", "att_1")
	err := world.Attachments.GetAccess(world.MemberBearer("bo"), "att_1")
	Assert.IsTrue(err == nil, "a reader of the message was refused")
}

func (test *AttachmentsControllerTest) TestGetAccessHidesTheAttachmentFromOthers() {
	world := MessagingTestSetup{}.World()
	world.AddMember("cy")
	world.WithAttachment("bo", "att_1")
	other := world.Attachments.GetAccess(world.MemberBearer("cy"), "att_1")
	unknown := world.Attachments.GetAccess(world.MemberBearer("bo"), "att_2")
	stranger := world.Attachments.GetAccess(world.StrangerBearer("bo"), "att_1")
	Assert.IsTrue(world.IsNotFound(other), "another member: want NotFound")
	Assert.IsTrue(world.IsNotFound(unknown), "an unknown attachment: want NotFound")
	Assert.IsTrue(world.IsNotFound(stranger), "another domain: want NotFound")
}

func (test *AttachmentsControllerTest) TestGetAccessRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	err := world.Attachments.GetAccess("", "att_1")
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *AttachmentsControllerTest) TestGetAccessRefusesAServiceToken() {
	world := MessagingTestSetup{}.World()
	err := world.Attachments.GetAccess(world.ServiceBearer(world.Home), "att_1")
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}
