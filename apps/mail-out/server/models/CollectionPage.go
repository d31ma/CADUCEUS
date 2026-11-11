type CollectionPage struct {
	Entries []CollectionEntry `json:"entries"`
	Next    string            `json:"next"`
}
