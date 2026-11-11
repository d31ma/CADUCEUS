import "errors"

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentCompleteServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentCompleteServiceTest) TestCompleteRefusesACallerWithoutAToken() {
	_, err := AttachmentsTestSetup{}.Completer().Complete("Bearer forged", "att_1")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}

func (test *AttachmentCompleteServiceTest) TestCompleteMarksTheUploadReady() {
	caller := AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana")
	upload, _ := AttachmentsTestSetup{}.Uploaded(caller, "hello", "text/plain")
	attachment, err := AttachmentsTestSetup{}.Completer().complete(caller, upload.AttachmentId)
	Assert.IsTrue(err == nil, "complete failed")
	Assert.Equals("ready", attachment.State)
	Assert.Equals(upload.AttachmentId, attachment.AttachmentId)
	Assert.Equals(int64(5), attachment.Size)
}

func (test *AttachmentCompleteServiceTest) TestCompleteTwiceAnswersTheSameAttachment() {
	caller := AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana")
	upload, _ := AttachmentsTestSetup{}.Uploaded(caller, "hello", "text/plain")
	first, _ := AttachmentsTestSetup{}.Completer().complete(caller, upload.AttachmentId)
	second, err := AttachmentsTestSetup{}.Completer().complete(caller, upload.AttachmentId)
	Assert.IsTrue(err == nil, "second complete failed")
	Assert.Equals(first, second)
}

func (test *AttachmentCompleteServiceTest) TestCompleteByAnotherMemberIsNotFound() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload, _ := AttachmentsTestSetup{}.Uploaded(AttachmentsTestSetup{}.Member(domainId, "ana"), "hello", "text/plain")
	_, err := AttachmentsTestSetup{}.Completer().complete(AttachmentsTestSetup{}.Member(domainId, "bo"), upload.AttachmentId)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another member completed the upload")
}

func (test *AttachmentCompleteServiceTest) TestCompleteBeforeTheUploadIsAConflict() {
	AttachmentsTestSetup{}.Settings("")
	caller := AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana")
	upload, _ := AttachmentsTestSetup{}.Creator().create(caller, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	_, err := AttachmentsTestSetup{}.Completer().complete(caller, upload.AttachmentId)
	conflict := &AttachmentConflict{}
	Assert.IsTrue(errors.As(err, &conflict), "missing upload accepted")
	Assert.Equals("file", conflict.Field)
}

func (test *AttachmentCompleteServiceTest) TestCompleteOfAMissingAttachmentIsNotFound() {
	AttachmentsTestSetup{}.Settings("")
	_, err := AttachmentsTestSetup{}.Completer().complete(AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana"), "att_NOPE")
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "missing attachment found")
}
