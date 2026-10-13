type MailtoAddress struct {
	Valid          bool   `json:"valid"`
	Mailbox        string `json:"mailbox"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
	HasAuthority   bool   `json:"hasAuthority"`
	HasCredentials bool   `json:"hasCredentials"`
}
