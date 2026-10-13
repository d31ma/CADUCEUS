// MailSelected says a message row was opened in the list (the old
// caduceus:mail-select window event).
type MailSelected struct {
	EmailID string `json:"emailId"`
}
