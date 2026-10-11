// SearchSuggestionsWanted asks for the server-backed suggestions once typing
// has paused.
type SearchSuggestionsWanted struct {
	Value string `json:"value"`
	Caret int    `json:"caret"`
}
