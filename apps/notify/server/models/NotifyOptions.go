type NotifyOptions struct {
	Push     PushOptions     `json:"push"`
	Presence PresenceOptions `json:"presence"`
	Summary  SummaryOptions  `json:"summary"`
}
