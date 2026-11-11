type PersonPage struct {
	Items      []PersonSummary `json:"items"`
	NextCursor string          `json:"nextCursor"`
}
