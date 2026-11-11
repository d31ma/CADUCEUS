type DeliveryLine struct {
	Recipient    string `json:"recipient"`
	Channel      string `json:"channel"`
	ChannelLabel string `json:"channelLabel"`
	Status       string `json:"status"`
	StatusLabel  string `json:"statusLabel"`
	Tone         string `json:"tone"`
	Detail       string `json:"detail"`
}
