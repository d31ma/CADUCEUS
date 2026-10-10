// TrackingActivityOptions selects the activity feed of one mailbox owner.
// A nil From, To, Since or Limit is not given.
type TrackingActivityOptions struct {
	AllowedDomains []string
	OwnerEmail     string
	OwnedSenders   []string
	SentFolders    map[string]string
	DraftFolders   map[string]string
	// Type is "open", "click" or anything else for all events.
	Type  string
	From  *float64
	To    *float64
	Since *float64
	Limit *float64
}
