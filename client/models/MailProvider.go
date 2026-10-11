type MailProvider struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Provider       string          `json:"provider"`
	Authentication string          `json:"authentication"`
	Guidance       string          `json:"guidance"`
	Configured     bool            `json:"configured"`
	Preset         *ProviderPreset `json:"preset"`
}
