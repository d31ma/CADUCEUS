type FanoutRecipientsPatch struct {
	Recipients []string `json:"recipients"`
	State      string   `json:"state"`
}
