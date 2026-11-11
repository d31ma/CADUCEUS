package core

type EmailMeta struct {
	MessageIdHeader string   `json:"messageIdHeader"`
	InReplyTo       string   `json:"inReplyTo"`
	References      []string `json:"references"`
	Subject         string   `json:"subject"`
	From            string   `json:"from"`
	To              []string `json:"to"`
	Cc              []string `json:"cc"`
	Spf             string   `json:"spf"`
	Dkim            string   `json:"dkim"`
	Dmarc           string   `json:"dmarc"`
	Handoff         bool     `json:"handoff"`
	Suspicious      bool     `json:"suspicious"`
	InboundId       string   `json:"inboundId"`
	RawBlobKey      string   `json:"rawBlobKey"`
}
