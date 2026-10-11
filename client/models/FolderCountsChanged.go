type FolderCountsChanged struct {
	Counts              map[string]int            `json:"counts"`
	ByDomain            map[string]map[string]int `json:"byDomain"`
	ByAccount           map[string]map[string]int `json:"byAccount"`
	UnreadInbox         int                       `json:"unreadInbox"`
	UnreadInboxByDomain map[string]int            `json:"unreadInboxByDomain"`
}
