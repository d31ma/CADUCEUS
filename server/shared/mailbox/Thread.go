// Thread is one conversation: its messages oldest first, the latest one,
// its participants and the mailbox's personal level in it.
type Thread struct {
	Id            string           `json:"id"`
	Subject       string           `json:"subject"`
	Count         int              `json:"count"`
	LatestEmail   map[string]any   `json:"latestEmail"`
	Emails        []map[string]any `json:"emails"`
	Participants  []string         `json:"participants"`
	PersonalLevel int              `json:"personalLevel"`
}
