type ProviderPreset struct {
	Imap            MailEndpoint `json:"imap"`
	Smtp            MailEndpoint `json:"smtp"`
	ContainerFolder string       `json:"containerFolder"`
}
