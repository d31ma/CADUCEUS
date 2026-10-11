type ExternalAccount struct {
	ID              string               `json:"id"`
	MailboxAddress  string               `json:"mailboxAddress"`
	Provider        string               `json:"provider"`
	State           string               `json:"state"`
	Revision        int                  `json:"revision"`
	Label           string               `json:"label"`
	Color           string               `json:"color"`
	Order           int                  `json:"order"`
	Capabilities    ExternalCapabilities `json:"capabilities"`
	CanMoveUp       bool                 `json:"canMoveUp"`
	CanMoveDown     bool                 `json:"canMoveDown"`
	CanCreateFolder bool                 `json:"canCreateFolder"`
}
