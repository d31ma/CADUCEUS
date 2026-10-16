type ActivityMetrics struct {
	Sent         float64           `json:"sent"`
	Received     float64           `json:"received"`
	Tracked      float64           `json:"tracked"`
	Opened       float64           `json:"opened"`
	Clicked      float64           `json:"clicked"`
	Replied      float64           `json:"replied"`
	TrackingRate float64           `json:"trackingRate"`
	OpenRate     float64           `json:"openRate"`
	ClickRate    float64           `json:"clickRate"`
	ReplyRate    float64           `json:"replyRate"`
	BySubject    []ActivitySubject `json:"bySubject"`
	Daily        []ActivityDay     `json:"daily"`
}
