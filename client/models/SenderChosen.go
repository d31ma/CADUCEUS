type SenderChosen struct {
	Reason        string          `json:"reason"`
	Address       string          `json:"address"`
	Apply         bool            `json:"apply"`
	NextCc        []string        `json:"nextCc"`
	NextBlindCopy []string        `json:"nextBcc"`
	Preferences   MailPreferences `json:"preferences"`
}
