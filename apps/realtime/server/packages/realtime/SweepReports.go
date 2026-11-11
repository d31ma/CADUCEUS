package realtime

type SweepReports struct{}

func (SweepReports) Sum(first SweepReport, second SweepReport) SweepReport {
	return SweepReport{
		ClosedConnections: first.ClosedConnections + second.ClosedConnections,
		OfflineMembers:    first.OfflineMembers + second.OfflineMembers,
		EndedDoNotDisturb: first.EndedDoNotDisturb + second.EndedDoNotDisturb,
		DeletedTickets:    first.DeletedTickets + second.DeletedTickets,
		Writes:            first.Writes + second.Writes,
		More:              SweepReports{}.either(first.More, second.More),
	}
}

func (SweepReports) either(first bool, second bool) bool {
	return first || second
}
