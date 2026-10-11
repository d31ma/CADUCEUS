// ScheduledRecord is one owned scheduled message: its management record
// (document id and entry) and, for hosted delivery, its queue entry.
type ScheduledRecord struct {
	DocumentId      string
	Entry           map[string]any
	QueueDocumentId string
	Queue           map[string]any
}
