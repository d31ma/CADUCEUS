// A row was clicked, or Enter or Space pressed on it.
type MailRowActivated struct {
	EmailID  string `json:"emailId"`
	ThreadID string `json:"threadId"`
}
