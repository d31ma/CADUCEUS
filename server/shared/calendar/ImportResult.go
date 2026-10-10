// ImportResult counts what one message's invitations changed: events
// imported or updated, cancellations applied and replies applied.
type ImportResult struct {
	Imported     int `json:"imported"`
	Cancelled    int `json:"cancelled"`
	ReplyApplied int `json:"replyApplied"`
}
