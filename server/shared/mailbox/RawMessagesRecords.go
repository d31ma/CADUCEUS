// RawMessagesRecords stages exact message sources for FYLO's file bucket.
type RawMessagesRecords struct{}

// BucketId answers the bucket file id of one source record.
func (RawMessagesRecords) BucketId(record map[string]any) string {
	return TextValue{}.Of(record["bucketId"])
}

// DocumentId answers the document id of one source record.
func (RawMessagesRecords) DocumentId(record map[string]any) string {
	return TextValue{}.Of(record["docId"])
}

// WithStagedSource writes message bytes to a private temporary file beside
// the FYLO root, hands its path to store and removes it afterwards.
func (RawMessagesRecords) WithStagedSource(fyloRoot string, content []byte, store func(path string) (string, error)) (string, error) {
	return IncomingFile{Prefix: "raw-message-", Name: "message.eml", Mode: 0o600}.Stored(fyloRoot, content, store)
}
