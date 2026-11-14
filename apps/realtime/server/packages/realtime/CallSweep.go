package realtime

import "time"

type CallSweep struct {
	Calls CallBook
	Seats SeatBook
}

func (sweep CallSweep) Clean(domainId string, now time.Time, limit int) (SweepReport, error) {
	stale, err := sweep.stale(domainId, now, limit)
	if err != nil {
		return SweepReport{}, err
	}
	return sweep.endAll(domainId, stale, limit)
}

func (sweep CallSweep) stale(domainId string, now time.Time, limit int) ([]CallRecord, error) {
	if limit <= 0 {
		return []CallRecord{}, nil
	}
	return sweep.Calls.Unchanged(domainId, now.Add(-SignalLimits{}.StaleCall()))
}

func (sweep CallSweep) endAll(domainId string, stale []CallRecord, limit int) (SweepReport, error) {
	report := SweepReport{}
	for _, record := range stale {
		ended, err := sweep.endOne(domainId, record, limit-report.Writes)
		report = SweepReports{}.Sum(report, ended)
		if err != nil || ended.More {
			return report, err
		}
	}
	return report, nil
}

func (sweep CallSweep) endOne(domainId string, record CallRecord, room int) (SweepReport, error) {
	seats, err := sweep.Seats.Seats(domainId, record.CallId)
	if err != nil {
		return SweepReport{}, err
	}
	return sweep.removed(domainId, record, seats, room)
}

func (sweep CallSweep) removed(domainId string, record CallRecord, seats []SeatRecord, room int) (SweepReport, error) {
	writes := len(seats) + 1
	if writes > room {
		return SweepReport{More: true}, nil
	}
	err := sweep.Seats.Remove(domainId, seats)
	if err != nil {
		return SweepReport{}, err
	}
	return SweepReport{EndedCalls: 1, Writes: writes}, sweep.Calls.Delete(domainId, record)
}
