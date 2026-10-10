type ProviderRedirectRequested struct {
	URL        string `json:"url"`
	Provider   string `json:"provider"`
	State      string `json:"state"`
	Onboarding bool   `json:"onboarding"`
}
