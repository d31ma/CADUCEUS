// SendPlan is one accepted POST /send once validated: the caller, the
// request to deliver, the resolved sender (and connected account, when the
// sender is one), the tracking id, the optional reminder, schedule and undo
// delay, the Send & Archive source, and the idempotency journal row.
type SendPlan struct {
	Caller          RouteCaller
	Request         map[string]any
	Sender          string
	Account         map[string]any
	Recipients      []string
	TrackingId      string
	RemindAt        int64
	HasReminder     bool
	SendAt          int64
	HasSendAt       bool
	DelayMs         int64
	ArchivePlan     map[string]any
	ArchiveSourceId string
	IdempotencyKey  string
	JournalId       string
}
