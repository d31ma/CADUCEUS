type Presence struct {
	MemberId   string `json:"memberId,omitempty"`
	State      string `json:"state"`
	DndUntil   string `json:"dndUntil,omitempty"`
	LastSeenAt string `json:"lastSeenAt"`
}
