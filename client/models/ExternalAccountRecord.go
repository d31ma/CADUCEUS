type ExternalAccountRecord struct {
	ID                   string              `json:"id"`
	MailboxAddress       string              `json:"mailboxAddress"`
	Provider             string              `json:"provider"`
	State                string              `json:"state"`
	Revision             float64             `json:"revision"`
	AuthIdentity         string              `json:"authIdentity"`
	SharedMailboxAddress string              `json:"sharedMailboxAddress"`
	Label                string              `json:"label"`
	Color                string              `json:"color"`
	Order                float64             `json:"order"`
	Capabilities         AccountCapabilities `json:"capabilities"`
	Connection           AccountConnection   `json:"connection"`
	Progress             *AccountProgress    `json:"progress"`
	Failure              *AccountFailure     `json:"failure"`
	UpdatedAt            string              `json:"updatedAt"`
}
