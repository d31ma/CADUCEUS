package realtime

type TicketSweep struct {
	Book TicketBook
}

func (sweep TicketSweep) Clean(limit int) (SweepReport, error) {
	stale, err := sweep.stale(limit)
	if err != nil {
		return SweepReport{}, err
	}
	return sweep.removeAll(stale)
}

func (sweep TicketSweep) stale(limit int) ([]TicketRecord, error) {
	if limit <= 0 {
		return []TicketRecord{}, nil
	}
	return sweep.Book.Stale(limit)
}

func (sweep TicketSweep) removeAll(records []TicketRecord) (SweepReport, error) {
	report := SweepReport{}
	for _, record := range records {
		err := sweep.Book.Remove(record)
		if err != nil {
			return report, err
		}
		report = SweepReports{}.Sum(report, SweepReport{DeletedTickets: 1, Writes: 1})
	}
	return report, nil
}
