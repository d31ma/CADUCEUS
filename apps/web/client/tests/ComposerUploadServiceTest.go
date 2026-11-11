// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerUploadServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerUploadServiceTest) uploads() *ComposerUploadService {
	return ComposerTestSetup{}.Uploads(test.behaviors)
}

func (test *ComposerUploadServiceTest) megabytes(count int64) int64 {
	return count * 1024 * 1024
}

func (test *ComposerUploadServiceTest) TestPickCreatesTheUpload() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", MediaType: "application/pdf", Size: test.megabytes(2), Sha256: "abc"})
	Assert.Equals("uploading", uploads.Items[0].State)
	Assert.Equals("att-plan.pdf", uploads.Items[0].AttachmentId)
	Assert.Equals("2.0 MB", uploads.Items[0].SizeText)
}

func (test *ComposerUploadServiceTest) TestPickRefusesAFileOverTheFileLimit() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "video.mov", Size: test.megabytes(26)})
	Assert.Equals("refused", uploads.Items[0].State)
	Assert.Equals("fileTooLarge", uploads.Items[0].Problem)
}

func (test *ComposerUploadServiceTest) TestPickRefusesAFileOverTheMessageLimit() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "a.pdf", Size: test.megabytes(20)})
	uploads.Pick(UploadPicked{FileName: "b.pdf", Size: test.megabytes(20)})
	uploads.Pick(UploadPicked{FileName: "c.pdf", Size: test.megabytes(20)})
	Assert.Equals("messageTooLarge", uploads.Items[2].Problem)
	Assert.Equals("40.0 MB of 50.0 MB used", uploads.UsedText())
}

func (test *ComposerUploadServiceTest) TestPickMarksAFailedCreate() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "broken.pdf", Size: 10})
	Assert.Equals("failed", uploads.Items[0].State)
	Assert.Equals(1, uploads.CountInState("failed"))
}

func (test *ComposerUploadServiceTest) TestCompleteMakesTheFileReady() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", Size: 10})
	uploads.Complete("att-plan.pdf")
	Assert.Equals("ready", uploads.Items[0].State)
	Assert.Equals([]string{"att-plan.pdf"}, uploads.AttachmentIds())
}

func (test *ComposerUploadServiceTest) TestAttachmentIdsLeaveOutUnfinishedFiles() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", Size: 10})
	Assert.Equals(0, len(uploads.AttachmentIds()))
}

func (test *ComposerUploadServiceTest) TestRetryCreatesTheUploadAgain() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", Size: 10})
	uploads.Items[0].State = "failed"
	uploads.Retry("plan.pdf")
	Assert.Equals(1, len(uploads.Items))
	Assert.Equals("uploading", uploads.Items[0].State)
}

func (test *ComposerUploadServiceTest) TestRemoveDropsTheFile() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", Size: 10})
	uploads.Remove("plan.pdf")
	Assert.Equals(0, len(uploads.Items))
}

func (test *ComposerUploadServiceTest) TestClearDropsEveryFile() {
	uploads := test.uploads()
	uploads.Pick(UploadPicked{FileName: "plan.pdf", Size: 10})
	uploads.Clear()
	Assert.Equals("0.0 MB of 50.0 MB used", uploads.UsedText())
}
