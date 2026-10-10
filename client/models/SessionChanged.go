type SessionChanged struct {
	Token            string                      `json:"token"`
	Email            string                      `json:"email"`
	Role             string                      `json:"role"`
	Domains          []string                    `json:"domains"`
	MailboxNames     map[string]string           `json:"mailboxNames"`
	ExternalAccounts []ExternalAccount           `json:"externalAccounts"`
	ExternalFolders  map[string][]ExternalFolder `json:"externalFolders"`
}
