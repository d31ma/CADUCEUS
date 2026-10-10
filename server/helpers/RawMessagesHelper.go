import "fmt"

// RawMessagesHelper keeps the exact RFC 822 source of each message in the
// tenant's raw-message bucket.
// @Helper
type RawMessagesHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// SaveForEmail stores the exact bytes of a message, replacing any source the
// email had, and answers the stored record.
func (helper *RawMessagesHelper) SaveForEmail(emailId string, domain string, source []byte) (map[string]any, error) {
	database := helper.fyloHelper.Database()
	if err := helper.DeleteForEmail(emailId); err != nil {
		return nil, err
	}
	random := RandomText{}
	id := random.Hex(16)
	size := len(source)
	bucket := FyloCollections{}.Named().RawMessageFiles
	bucketId, err := RawMessagesRecords{}.WithStagedSource(database.Root, source, func(path string) (string, error) {
		key := fmt.Sprintf("/emails/%s/source.eml", random.Digest([]byte(emailId)))
		return database.PutFile(bucket, FyloFile{Path: path, Key: key, MaxBytes: &size, Meta: map[string]any{"domain": domain}})
	})
	if err != nil {
		return nil, err
	}
	record := map[string]any{
		"id": id, "emailId": emailId, "domain": domain, "bucketId": bucketId,
		"size": size, "sha256": random.Digest(source), "createdAt": random.Now(),
	}
	if _, err := helper.messages().Put(record); err != nil {
		_ = database.DelDoc(bucket, bucketId)
		return nil, err
	}
	return record, nil
}

// FindForEmail answers the source record of one email with its docId, or nil.
func (helper *RawMessagesHelper) FindForEmail(emailId string) (map[string]any, error) {
	record, err := helper.messages().First("emailId", emailId)
	if err != nil || record.Id == "" {
		return nil, err
	}
	return record.WithDocId(), nil
}

// ReadContent answers the stored bytes of one source record.
func (helper *RawMessagesHelper) ReadContent(record map[string]any) ([]byte, error) {
	return helper.fyloHelper.Database().GetFileBytes(FyloCollections{}.Named().RawMessageFiles, RawMessagesRecords{}.BucketId(record))
}

// DeleteForEmail deletes one email's source record and its bucket file.
func (helper *RawMessagesHelper) DeleteForEmail(emailId string) error {
	record, err := helper.FindForEmail(emailId)
	if err != nil || record == nil {
		return err
	}
	if err := helper.messages().Delete(RawMessagesRecords{}.DocumentId(record)); err != nil {
		return err
	}
	return helper.fyloHelper.Database().DelDoc(FyloCollections{}.Named().RawMessageFiles, RawMessagesRecords{}.BucketId(record))
}

func (helper *RawMessagesHelper) messages() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().RawMessages}
}
