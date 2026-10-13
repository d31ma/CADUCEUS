type DiscoveredConnection struct {
	Imap            MailEndpoint `json:"imap"`
	Smtp            MailEndpoint `json:"smtp"`
	Carddav         DavEndpoint  `json:"carddav"`
	Caldav          DavEndpoint  `json:"caldav"`
	ContainerFolder string       `json:"containerFolder"`
}
