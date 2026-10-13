type ConnectionDraft struct {
	ChoiceID             string `json:"choiceId"`
	Source               string `json:"source"`
	MailboxAddress       string `json:"mailboxAddress"`
	Label                string `json:"label"`
	Username             string `json:"username"`
	ImapHost             string `json:"imapHost"`
	ImapPort             string `json:"imapPort"`
	ImapSecurity         string `json:"imapSecurity"`
	SmtpHost             string `json:"smtpHost"`
	SmtpPort             string `json:"smtpPort"`
	SmtpSecurity         string `json:"smtpSecurity"`
	CardDavURL           string `json:"cardDAVURL"`
	CardDavUsername      string `json:"cardDAVUsername"`
	CalDavURL            string `json:"calDAVURL"`
	CalDavUsername       string `json:"calDAVUsername"`
	ContainerFolder      string `json:"containerFolder"`
	OauthProvider        string `json:"oauthProvider"`
	AuthIdentity         string `json:"authIdentity"`
	SharedMailboxAddress string `json:"sharedMailboxAddress"`
	Color                string `json:"color"`
}
