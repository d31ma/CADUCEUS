package realtime

type SweepReport struct {
	ClosedConnections int
	OfflineMembers    int
	EndedDoNotDisturb int
	Writes            int
	More              bool
}
