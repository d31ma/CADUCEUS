// ProviderLaunch is what the settings address carried when the page opened:
// a provider's OAuth redirect, or a request to start the account setup.
type ProviderLaunch struct {
	Onboarding  bool   `json:"onboarding"`
	HasCallback bool   `json:"hasCallback"`
	Failed      bool   `json:"failed"`
	Code        string `json:"code"`
	State       string `json:"state"`
	Provider    string `json:"provider"`
	Cleaned     string `json:"cleaned"`
}
