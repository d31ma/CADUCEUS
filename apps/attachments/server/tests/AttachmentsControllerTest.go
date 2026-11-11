import "errors"

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentsControllerTest) controller() *AttachmentsController {
	return &AttachmentsController{attachmentCreateService: AttachmentsTestSetup{}.Creator(), attachmentCompleteService: AttachmentsTestSetup{}.Completer(), attachmentDownloadService: AttachmentsTestSetup{}.Downloader()}
}

func (test *AttachmentsControllerTest) TestPostAttachmentFailsClosedWithoutAValidToken() {
	_, err := test.controller().PostAttachment("Bearer forged", AttachmentsTestSetup{}.Request("hello", "text/plain"))
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}

func (test *AttachmentsControllerTest) TestPostCompleteFailsClosedWithoutAValidToken() {
	_, err := test.controller().PostComplete("Bearer forged", "att_1")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}

func (test *AttachmentsControllerTest) TestGetAttachmentFailsClosedWithoutAValidToken() {
	_, err := test.controller().GetAttachment("Bearer forged", "att_1")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}
