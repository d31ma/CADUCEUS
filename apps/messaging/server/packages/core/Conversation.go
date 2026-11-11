package core

type Conversation struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	Key            string `json:"key"`
	Kind           string `json:"kind"`
	ChannelId      string `json:"channelId"`
	ChannelName    string `json:"channelName"`
	Visibility     string `json:"visibility"`
	EmailAddress   string `json:"emailAddress"`
	Archived       bool   `json:"archived"`
	Subject        string `json:"subject"`
	HasExternal    bool   `json:"hasExternal"`
	ParticipantKey string `json:"participantKey"`
	MessageCount   int    `json:"messageCount"`
	CreatedMonth   string `json:"createdMonth"`
	LastMessageAt  string `json:"lastMessageAt"`
	CreatedAt      string `json:"createdAt"`
}
