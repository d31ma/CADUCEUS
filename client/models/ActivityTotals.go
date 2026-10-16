type ActivityTotals struct {
	Sent              float64 `json:"sent"`
	Received          float64 `json:"received"`
	Tracked           float64 `json:"tracked"`
	Opened            float64 `json:"opened"`
	Clicked           float64 `json:"clicked"`
	Replied           float64 `json:"replied"`
	TrackingRateLabel string  `json:"trackingRateLabel"`
	OpenRateLabel     string  `json:"openRateLabel"`
	ClickRateLabel    string  `json:"clickRateLabel"`
	ReplyRateLabel    string  `json:"replyRateLabel"`
}
