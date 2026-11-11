import "errors"

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentMetaControllerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentMetaControllerTest) uploaded(domainId string) Upload {
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload, _ := (&AttachmentsController{}).PostAttachment(ana, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	AttachmentsTestSetup{}.Store(domainId, upload.AttachmentId, "hello")
	return upload
}

func (test *AttachmentMetaControllerTest) TestAServiceOfTheDomainReadsAReadyAttachment() {
	domainId := AttachmentsTestSetup{}.Domain()
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload, _ := (&AttachmentsController{}).PostAttachment(ana, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	AttachmentsTestSetup{}.Store(domainId, upload.AttachmentId, "hello")
	completed, _ := (&AttachmentsController{}).PostComplete(ana, upload.AttachmentId)
	attachment, err := (&AttachmentMetaController{}).GetMeta(AttachmentsTestSetup{}.SignedInService(domainId), upload.AttachmentId)
	Assert.IsTrue(err == nil, "meta failed")
	Assert.Equals(completed, attachment)
	Assert.Equals("ready", attachment.State)
	Assert.Equals("plan.txt", attachment.FileName)
	Assert.Equals(int64(5), attachment.Size)
}

func (test *AttachmentMetaControllerTest) TestAServiceOfTheDomainReadsAPendingUpload() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload := test.uploaded(domainId)
	attachment, err := (&AttachmentMetaController{}).GetMeta(AttachmentsTestSetup{}.SignedInService(domainId), upload.AttachmentId)
	Assert.IsTrue(err == nil, "meta failed")
	Assert.Equals("pending", attachment.State)
	Assert.Equals(upload.AttachmentId, attachment.AttachmentId)
	Assert.Equals("text/plain", attachment.MediaType)
}

func (test *AttachmentMetaControllerTest) TestAServiceOfAnotherDomainGetsNotFound() {
	upload := test.uploaded(AttachmentsTestSetup{}.Domain())
	_, err := (&AttachmentMetaController{}).GetMeta(AttachmentsTestSetup{}.SignedInService(AttachmentsTestSetup{}.Domain()), upload.AttachmentId)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another domain read the details")
}

func (test *AttachmentMetaControllerTest) TestAnUnknownAttachmentIsNotFound() {
	service := AttachmentsTestSetup{}.SignedInService(AttachmentsTestSetup{}.Domain())
	_, err := (&AttachmentMetaController{}).GetMeta(service, "att_unknown")
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "an unknown attachment was found")
}

func (test *AttachmentMetaControllerTest) TestAMemberIsForbidden() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload := test.uploaded(domainId)
	_, err := (&AttachmentMetaController{}).GetMeta(AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret"), upload.AttachmentId)
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As(err, &forbidden), "a member read the details")
}

func (test *AttachmentMetaControllerTest) TestAForgedTokenIsRefused() {
	AttachmentsTestSetup{}.SignedInService(AttachmentsTestSetup{}.Domain())
	_, err := (&AttachmentMetaController{}).GetMeta("Bearer forged", "att_unknown")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}
