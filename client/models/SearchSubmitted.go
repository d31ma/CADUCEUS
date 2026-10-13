// The search box asks the list to search, or with ResetStatus to clear the search.
type SearchSubmitted struct {
	Query       string `json:"query"`
	ResetStatus bool   `json:"resetStatus"`
}
