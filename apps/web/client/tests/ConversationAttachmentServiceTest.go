// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationAttachmentServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationAttachmentServiceTest) card(mediaType string, size int64) AttachmentCard {
	return (&ConversationAttachmentService{}).Cards([]AttachmentRef{{AttachmentId: "a1", FileName: "file", MediaType: mediaType, Size: size}})[0]
}

func (test *ConversationAttachmentServiceTest) TestCardCopiesIdAndFileName() {
	cards := (&ConversationAttachmentService{}).Cards([]AttachmentRef{{AttachmentId: "a1", FileName: "contract-signed.pdf", MediaType: "application/pdf", Size: 2516582}})
	Assert.Equals(1, len(cards))
	Assert.Equals("a1", cards[0].AttachmentId)
	Assert.Equals("contract-signed.pdf", cards[0].FileName)
	Assert.Equals("PDF", cards[0].TypeLabel)
	Assert.Equals("2.4 MB", cards[0].SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestNoAttachmentsGiveNoCards() {
	Assert.Equals(0, len((&ConversationAttachmentService{}).Cards(nil)))
}

func (test *ConversationAttachmentServiceTest) TestSmallSizeIsInBytes() {
	Assert.Equals("512 B", test.card("application/pdf", 512).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestEmptyFileIsZeroBytes() {
	Assert.Equals("0 B", test.card("application/pdf", 0).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestLargestByteSize() {
	Assert.Equals("1023 B", test.card("application/pdf", 1023).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestKilobytesHaveNoDecimals() {
	Assert.Equals("12 KB", test.card("application/pdf", 12288).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestMegabytesHaveOneDecimal() {
	Assert.Equals("3.4 MB", test.card("application/pdf", 3565158).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestGigabytesHaveOneDecimal() {
	Assert.Equals("1.5 GB", test.card("application/pdf", 1610612736).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestHugeSizeStaysInGigabytes() {
	Assert.Equals("2048.0 GB", test.card("application/pdf", 2199023255552).SizeLabel)
}

func (test *ConversationAttachmentServiceTest) TestImageType() {
	Assert.Equals("Image", test.card("image/png", 1).TypeLabel)
}

func (test *ConversationAttachmentServiceTest) TestTextTypeIgnoresParameters() {
	Assert.Equals("Text", test.card("Text/Plain; charset=utf-8", 1).TypeLabel)
}

func (test *ConversationAttachmentServiceTest) TestSpreadsheetTypes() {
	Assert.Equals("Spreadsheet", test.card("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", 1).TypeLabel)
	Assert.Equals("Spreadsheet", test.card("text/csv", 1).TypeLabel)
}

func (test *ConversationAttachmentServiceTest) TestArchiveType() {
	Assert.Equals("Archive", test.card("application/zip", 1).TypeLabel)
}

func (test *ConversationAttachmentServiceTest) TestOtherTypeIsFile() {
	Assert.Equals("File", test.card("application/octet-stream", 1).TypeLabel)
	Assert.Equals("File", test.card("", 1).TypeLabel)
}
