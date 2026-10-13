// ExtensionsChanged says the installed extensions changed in Settings, so the
// runtime loads them again.
type ExtensionsChanged struct {
	Reason string `json:"reason"`
}
