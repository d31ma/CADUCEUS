type SearchProgress struct {
	Spot    FindResult   `json:"spot"`
	Next    int          `json:"next"`
	Results []FindResult `json:"results"`
}
