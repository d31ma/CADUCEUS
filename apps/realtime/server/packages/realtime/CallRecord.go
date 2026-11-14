package realtime

type CallRecord struct {
	ID              string `json:"id,omitempty"`
	CallId          string `json:"callId"`
	Group           bool   `json:"group"`
	Media           string `json:"media"`
	CreatedByDevice string `json:"createdByDevice"`
	CreatedByPerson string `json:"createdByPerson"`
	CreatedAt       string `json:"createdAt"`
	LastChangeAt    string `json:"lastChangeAt"`
}
