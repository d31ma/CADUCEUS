// ScheduledEdit is one in-place edit of a scheduled message: its management
// record (document id and entry), its queue document, the owner and sender,
// the new request and the new send time (milliseconds).
type ScheduledEdit struct {
	ScheduledDocumentId string
	Entry               map[string]any
	QueueDocumentId     string
	Owner               string
	Sender              string
	Request             map[string]any
	SendAt              int64
}
