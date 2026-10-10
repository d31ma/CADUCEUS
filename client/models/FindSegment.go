type FindSegment struct {
	Text    string `json:"text"`
	Match   bool   `json:"match"`
	Index   int    `json:"index"`
	Current bool   `json:"current"`
}
