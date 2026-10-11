type SearchSuggestionRow struct {
	ID          string `json:"id"`
	Index       int    `json:"index"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ClassName   string `json:"className"`
	Selected    string `json:"selected"`
}
