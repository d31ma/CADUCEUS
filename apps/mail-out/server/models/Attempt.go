type Attempt struct {
	Item       OutboundItem `json:"item"`
	Now        string       `json:"now"`
	Suppressed bool         `json:"suppressed"`
	Free       bool         `json:"free"`
}
