package realtime

type SweepReport struct {
	ClosedConnections int
	OfflineMembers    int
	EndedDoNotDisturb int
	DeletedTickets    int
	Writes            int
	More              bool
}
