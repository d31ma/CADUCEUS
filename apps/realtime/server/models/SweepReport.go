type SweepReport struct {
	ClosedConnections int  `json:"closedConnections"`
	OfflineMembers    int  `json:"offlineMembers"`
	EndedDoNotDisturb int  `json:"endedDoNotDisturb"`
	DeletedTickets    int  `json:"deletedTickets"`
	EndedCalls        int  `json:"endedCalls"`
	Writes            int  `json:"writes"`
	More              bool `json:"more"`
}
