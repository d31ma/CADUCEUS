type ProviderLaunch struct {
	Onboarding  bool   `json:"onboarding"`
	HasCallback bool   `json:"hasCallback"`
	Failed      bool   `json:"failed"`
	Code        string `json:"code"`
	State       string `json:"state"`
	Provider    string `json:"provider"`
}
