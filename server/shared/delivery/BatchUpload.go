// BatchUpload is one per-recipient upload into a provider submission
// journal: the accounts transport and account, the operation family
// ("accounts.submit_batch" or "accounts.submit_batch_update"), the current
// submission (and its id when the submission answer does not carry it), and
// the message to render for each recipient.
type BatchUpload struct {
	Accounts     ProviderAccounts
	AccountId    string
	Operation    string
	Submission   map[string]any
	SubmissionId string
	Request      map[string]any
	TrackingId   string
	Sender       string
	MessageId    string
	Recipients   []string
}
