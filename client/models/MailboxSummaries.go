// Loaded message summaries for the encrypted offline index, in slices under 64 KiB.
type MailboxSummaries struct {
	Owner  string  `json:"owner"`
	Emails []Email `json:"emails"`
}
