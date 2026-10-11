// TrackingHit is one open or click of a sent message, as GET /inbox/{id}
// lists them in opens and clicks.
type TrackingHit struct {
	OpenedAt  string `json:"openedAt"`
	ClickedAt string `json:"clickedAt"`
	Recipient string `json:"recipient"`
	Title     string `json:"title"`
	Url       string `json:"url"`
}
