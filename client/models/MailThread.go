// A conversation as /threads returns it.
type MailThread struct {
	ID            string  `json:"id"`
	Subject       string  `json:"subject"`
	PersonalLevel int     `json:"personalLevel"`
	Emails        []Email `json:"emails"`
}
