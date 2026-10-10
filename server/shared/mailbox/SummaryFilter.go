// SummaryFilter selects provider index rows; nil flags leave a filter off.
type SummaryFilter struct {
	AccountId      string
	FolderRole     string
	FolderStableId string
	Query          string
	Read           *bool
	Starred        *bool
	ReceivedAtFrom *string
	OldestFirst    bool
	Limit          int
	Offset         int
}
