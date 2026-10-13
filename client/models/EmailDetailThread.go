// EmailDetailThread is a complete authorized conversation requested by email ID.
// MailThread remains the summary used by mailbox list routes.
type EmailDetailThread struct {
	ID            string        `json:"id"`
	Subject       string        `json:"subject"`
	PersonalLevel int           `json:"personalLevel"`
	Emails        []EmailDetail `json:"emails"`
}
