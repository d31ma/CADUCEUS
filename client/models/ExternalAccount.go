type ExternalAccount struct {
	ID                   string           `json:"id"`
	MailboxAddress       string           `json:"mailboxAddress"`
	Provider             string           `json:"provider"`
	State                string           `json:"state"`
	Revision             float64          `json:"revision"`
	AuthIdentity         string           `json:"authIdentity"`
	SharedMailboxAddress string           `json:"sharedMailboxAddress"`
	Label                string           `json:"label"`
	Color                string           `json:"color"`
	Order                float64          `json:"order"`
	CanReadMail          bool             `json:"canReadMail"`
	CanSend              bool             `json:"canSend"`
	ImapUsername         string           `json:"imapUsername"`
	Progress             *AccountProgress `json:"progress"`
	Failure              *AccountFailure  `json:"failure"`
	UpdatedAt            string           `json:"updatedAt"`
}
