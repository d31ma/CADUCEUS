type CreateChannelRequest struct {
	Name       string `json:"name"`
	Topic      string `json:"topic,omitempty"`
	Visibility string `json:"visibility"`
}
