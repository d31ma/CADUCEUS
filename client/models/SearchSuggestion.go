type SearchSuggestion struct {
	Token       string `json:"token"`
	Term        string `json:"term"`
	Description string `json:"description"`
	Submit      bool   `json:"submit"`
	Kind        string `json:"kind"`
}
