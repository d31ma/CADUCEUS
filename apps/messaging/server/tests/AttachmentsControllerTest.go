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
	Assert.Equals(1540, world.Number(other))
	Assert.IsTrue(world.IsNotFound(unknown), "an unknown attachment: want NotFound")
	Assert.Equals(1540, world.Number(unknown))
	Assert.IsTrue(world.IsNotFound(stranger), "another domain: want NotFound")
	Assert.Equals(1540, world.Number(stranger))
}

func (test *AttachmentsControllerTest) TestGetAccessRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	err := world.Attachments.GetAccess("", "att_1")
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
	Assert.Equals(1501, world.Number(err))
}

func (test *AttachmentsControllerTest) TestGetAccessRefusesAServiceToken() {
	world := MessagingTestSetup{}.World()
	err := world.Attachments.GetAccess(world.ServiceBearer(world.Home), "att_1")
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
	Assert.Equals(1502, world.Number(err))
}
