type BrowserFeatures struct {
	Passkeys      bool `json:"passkeys"`
	ServiceWorker bool `json:"serviceWorker"`
	LaunchQueue   bool `json:"launchQueue"`
	AppBadge      bool `json:"appBadge"`
	Notifications bool `json:"notifications"`
	PushDisabled  bool `json:"pushDisabled"`
}
