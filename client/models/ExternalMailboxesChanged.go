type ExternalMailboxesChanged struct {
	Accounts []ExternalAccount           `json:"accounts"`
	Folders  map[string][]ExternalFolder `json:"folders"`
}
