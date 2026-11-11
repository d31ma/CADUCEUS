type UpdateChannelRequest struct {
	Topic        *string `json:"topic,omitempty"`
	EmailAddress *string `json:"emailAddress,omitempty"`
	Archived     *bool   `json:"archived,omitempty"`
}
