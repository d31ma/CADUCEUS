package notify

type SummaryItem struct {
	Kind              string `json:"kind"`
	AuthorDisplayName string `json:"authorDisplayName"`
	CreatedAt         string `json:"createdAt"`
}
