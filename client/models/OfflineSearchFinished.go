// Offline index results, in slices; Done marks the last one.
type OfflineSearchFinished struct {
	Request int     `json:"request"`
	Emails  []Email `json:"emails"`
	Done    bool    `json:"done"`
}
