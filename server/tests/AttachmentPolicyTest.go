import "strings"

// @Test(MailboxTestSetup)
type AttachmentPolicyTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentPolicyTest) TestKeepsOnlyTheLastPathSegment() {
	Assert.Equals("a b.txt", AttachmentPolicy{}.SanitizeAttachmentFilename("../dir\\a  b‮.txt", "fallback"))
}

func (test *AttachmentPolicyTest) TestReplacesReservedCharacters() {
	Assert.Equals("a-b-c.txt", AttachmentPolicy{}.SanitizeAttachmentFilename("a:b*c.txt", "fallback"))
}

// @ValueSource(["", "...", "///", " . "])
func (test *AttachmentPolicyTest) TestFallsBackWhenNothingIsLeft(name string) {
	Assert.Equals("attachment-1", AttachmentPolicy{}.SanitizeAttachmentFilename(name, "attachment-1"))
}

func (test *AttachmentPolicyTest) TestCapsTheNameAt160Units() {
	Assert.Equals(160, len(AttachmentPolicy{}.SanitizeAttachmentFilename(strings.Repeat("abcdefghij", 20), "x")))
}

// @ValueSource(["<logo@example>", "cid:logo@example", " CID:<logo@example> "])
func (test *AttachmentPolicyTest) TestNormalizesAContentId(value string) {
	Assert.Equals("logo@example", AttachmentPolicy{}.NormalizeAttachmentContentId(value))
}

// @ValueSource(["", "<>", "a b", "a<b"])
func (test *AttachmentPolicyTest) TestRejectsAnUnsafeContentId(value string) {
	Assert.Equals("", AttachmentPolicy{}.NormalizeAttachmentContentId(value))
}

func (test *AttachmentPolicyTest) TestDecodesBase64LikeNode() {
	Assert.Equals("hello", string(Base64Text{}.Decoded("aGVs bG8=")))
	Assert.Equals("hello", string(Base64Text{}.Decoded("aGVsbG8")))
}

func (test *AttachmentPolicyTest) TestSummarizesARecord() {
	summary := AttachmentsRecords{}.ToSummary(map[string]any{"id": "a", "filename": "f", "bucketId": "b", "size": 3.0})
	Assert.Equals(map[string]any{"id": "a", "filename": "f", "size": 3.0}, summary)
}

func (test *AttachmentPolicyTest) TestRefusesAPathOutsideTheRoot() {
	records := AttachmentsRecords{}
	Assert.Equals(nil, records.EnsureInsideRoot("/data/attachments", "/data/attachments/a/b.bin"))
	Assert.IsTrue(records.EnsureInsideRoot("/data/attachments", "/data/other/b.bin") != nil)
}
