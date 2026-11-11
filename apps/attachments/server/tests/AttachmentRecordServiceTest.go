import "errors"

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentRecordServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentRecordServiceTest) stored(domainId string) AttachmentRecord {
	service := AttachmentsTestSetup{}.Records()
	collection, _ := service.Collection(domainId)
	record, _ := service.Insert(collection, AttachmentRecord{DomainId: domainId, UploaderId: "ana", FileName: "a.txt", MediaType: "text/plain", Size: 1, State: "pending", BlobKey: domainId + "/BLOB1"})
	return record
}

func (test *AttachmentRecordServiceTest) TestCollectionIsSafeToRepeat() {
	domainId := AttachmentsTestSetup{}.Domain()
	first, firstErr := AttachmentsTestSetup{}.Records().Collection(domainId)
	second, secondErr := AttachmentsTestSetup{}.Records().Collection(domainId)
	Assert.IsTrue(firstErr == nil && secondErr == nil, "collection failed")
	Assert.Equals(first, second)
}

func (test *AttachmentRecordServiceTest) TestInsertThenLookupFindsTheRecord() {
	domainId := AttachmentsTestSetup{}.Domain()
	record := test.stored(domainId)
	found, err := AttachmentsTestSetup{}.Records().Lookup(domainId, AttachmentsTestSetup{}.Records().WireId(record))
	Assert.IsTrue(err == nil, "lookup failed")
	Assert.Equals("a.txt", found.FileName)
	Assert.Equals(record.ID, found.ID)
	Assert.IsTrue(found.CreatedAt != "", "no creation time")
}

func (test *AttachmentRecordServiceTest) TestLookupFromAnotherDomainIsNotFound() {
	record := test.stored(AttachmentsTestSetup{}.Domain())
	_, err := AttachmentsTestSetup{}.Records().Lookup(AttachmentsTestSetup{}.Domain(), AttachmentsTestSetup{}.Records().WireId(record))
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another domain found the record")
}

func (test *AttachmentRecordServiceTest) TestLookupOfABadIdIsNotFound() {
	_, wrongPrefix := AttachmentsTestSetup{}.Records().Lookup("example.com", "msg_1")
	_, badCharacters := AttachmentsTestSetup{}.Records().Lookup("example.com", "att_../x")
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(wrongPrefix, &missing), "wrong prefix accepted")
	Assert.IsTrue(errors.As(badCharacters, &missing), "bad characters accepted")
}

func (test *AttachmentRecordServiceTest) TestSetStateSavesTheState() {
	domainId := AttachmentsTestSetup{}.Domain()
	record := test.stored(domainId)
	saved, err := AttachmentsTestSetup{}.Records().SetState(record, "ready")
	found, _ := AttachmentsTestSetup{}.Records().Lookup(domainId, AttachmentsTestSetup{}.Records().WireId(record))
	Assert.IsTrue(err == nil, "state not saved")
	Assert.Equals("ready", saved.State)
	Assert.Equals("ready", found.State)
}

func (test *AttachmentRecordServiceTest) TestViewShowsTheWireIdAndNoBlobKey() {
	view := AttachmentsTestSetup{}.Records().View(AttachmentRecord{ID: "X1", FileName: "a.txt", MediaType: "text/plain", Size: 3, Sha256: "abc", State: "ready", BlobKey: "example.com/B"})
	Assert.Equals(Attachment{AttachmentId: "att_X1", FileName: "a.txt", MediaType: "text/plain", Size: 3, Sha256: "abc", State: "ready"}, view)
}

func (test *AttachmentRecordServiceTest) TestKeySplitsTheBlobKeyAtTheDomain() {
	key := AttachmentsTestSetup{}.Records().Key(AttachmentRecord{BlobKey: "example.com/B1"})
	Assert.Equals("example.com", key.DomainId)
	Assert.Equals("B1", key.Name)
}
