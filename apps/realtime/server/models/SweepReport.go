type SweepReport struct {
	ClosedConnections int  `json:"closedConnections"`
	OfflineMembers    int  `json:"offlineMembers"`
	EndedDoNotDisturb int  `json:"endedDoNotDisturb"`
	Writes            int  `json:"writes"`
	More              bool `json:"more"`
}
