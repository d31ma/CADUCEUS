type ResolvedRecipient struct {
	Address           string   `json:"address"`
	DisplayName       string   `json:"displayName"`
	Kind              string   `json:"kind"`
	MemberId          string   `json:"memberId,omitempty"`
	ChannelId         string   `json:"channelId,omitempty"`
	DefaultDelivery   string   `json:"defaultDelivery"`
	AllowedDeliveries []string `json:"allowedDeliveries"`
}
