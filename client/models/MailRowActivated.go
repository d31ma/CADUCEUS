// A row was clicked, or Enter or Space pressed on it. Narrow says the window
// is phone-sized, where opening a message leaves for its own page.
type MailRowActivated struct {
	EmailID  string `json:"emailId"`
	ThreadID string `json:"threadId"`
	Narrow   bool   `json:"narrow"`
}
