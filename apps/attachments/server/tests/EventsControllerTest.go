import "errors"

// @Test(BackupsTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestAMemberMayNotSendEvents() {
	domainId := BackupsTestSetup{}.Domain()
	ana := BackupsTestSetup{}.Tokens(domainId, []string{"ana"})["ana"]
	_, err := BackupsTestSetup{}.EventsController().PostEvent(ana, BackupsTestSetup{}.MemberEvent("e1", domainId, "ana", "removed"))
	refused := &Forbidden{}
	Assert.IsTrue(errors.As(err, &refused), "a member sent an event")
	Assert.Equals(3521, refused.Number())
}

func (test *EventsControllerTest) TestAServiceMaySendEventsOnlyForItsDomain() {
	service := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{})["service"]
	_, err := BackupsTestSetup{}.EventsController().PostEvent(service, BackupsTestSetup{}.MemberEvent("e1", BackupsTestSetup{}.Domain(), "ana", "removed"))
	refused := &Forbidden{}
	Assert.IsTrue(errors.As(err, &refused), "an event for another domain was accepted")
	Assert.Equals(3522, refused.Number())
}

func (test *EventsControllerTest) TestABadEnvelopeIsInvalid() {
	domainId := BackupsTestSetup{}.Domain()
	service := BackupsTestSetup{}.Tokens(domainId, []string{})["service"]
	envelope := BackupsTestSetup{}.MemberEvent("", domainId, "ana", "removed")
	_, err := BackupsTestSetup{}.EventsController().PostEvent(service, envelope)
	invalid := &InvalidUpload{}
	Assert.IsTrue(errors.As(err, &invalid), "an envelope without an id was accepted")
	Assert.Equals(3523, invalid.Number())
}

func (test *EventsControllerTest) TestAServiceRemovesTheBackupOfARemovedMember() {
	domainId := BackupsTestSetup{}.Domain()
	tokens := BackupsTestSetup{}.Tokens(domainId, []string{"ana"})
	upload, _ := BackupsTestSetup{}.Controller().PutMine(tokens["ana"], BackupsTestSetup{}.Start(0, "cipher", "phone"))
	BackupsTestSetup{}.Put(upload.UploadUrl, "cipher")
	BackupsTestSetup{}.Controller().PostComplete(tokens["ana"], BackupComplete{Version: 1})
	receipt, err := BackupsTestSetup{}.EventsController().PostEvent(tokens["service"], BackupsTestSetup{}.MemberEvent("e-removed", domainId, "ana", "removed"))
	_, mineErr := BackupsTestSetup{}.Controller().GetMine(tokens["ana"])
	missing := &AttachmentNotFound{}
	Assert.IsTrue(err == nil, "the event failed")
	Assert.Equals("applied", receipt.Status)
	Assert.IsTrue(errors.As(mineErr, &missing), "the backup is still there")
	Assert.IsTrue(!BackupsTestSetup{}.Exists(upload.UploadUrl), "the bytes are still there")
}

func (test *EventsControllerTest) TestAnActiveMemberKeepsTheBackup() {
	domainId := BackupsTestSetup{}.Domain()
	tokens := BackupsTestSetup{}.Tokens(domainId, []string{"ana"})
	upload, _ := BackupsTestSetup{}.Controller().PutMine(tokens["ana"], BackupsTestSetup{}.Start(0, "cipher", "phone"))
	BackupsTestSetup{}.Put(upload.UploadUrl, "cipher")
	BackupsTestSetup{}.Controller().PostComplete(tokens["ana"], BackupComplete{Version: 1})
	_, err := BackupsTestSetup{}.EventsController().PostEvent(tokens["service"], BackupsTestSetup{}.MemberEvent("e-active", domainId, "ana", "active"))
	mine, mineErr := BackupsTestSetup{}.Controller().GetMine(tokens["ana"])
	Assert.Equals(nil, err)
	Assert.Equals(nil, mineErr)
	Assert.Equals(int64(1), mine.Version)
}
