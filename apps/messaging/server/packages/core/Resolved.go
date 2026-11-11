package core

type Resolved struct {
	Address     string `json:"address"`
	DisplayName string `json:"displayName"`
	Kind        string `json:"kind"`
	MemberId    string `json:"memberId"`
	ChannelId   string `json:"channelId"`
}
