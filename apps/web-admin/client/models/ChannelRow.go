type ChannelRow struct {
	ChannelId      string `json:"channelId"`
	Name           string `json:"name"`
	Topic          string `json:"topic"`
	NameIcon       string `json:"nameIcon"`
	VisibilityText string `json:"visibilityText"`
	VisibilityTag  string `json:"visibilityTag"`
	VisibilityIcon string `json:"visibilityIcon"`
	HasAddress     bool   `json:"hasAddress"`
	EmailAddress   string `json:"emailAddress"`
	AddressAction  string `json:"addressAction"`
	AddressLabel   string `json:"addressLabel"`
	Archived       bool   `json:"archived"`
	ArchiveLabel   string `json:"archiveLabel"`
	RestoreLabel   string `json:"restoreLabel"`
	RowClass       string `json:"rowClass"`
}
