type ActivityDay struct {
	Date     string  `json:"date"`
	Sent     float64 `json:"sent"`
	Received float64 `json:"received"`
	Opens    float64 `json:"opens"`
	Clicks   float64 `json:"clicks"`
}
