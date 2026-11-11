package realtime

import (
	"errors"
	"time"
)

type DomainSweep struct {
	Book        DomainBook
	Connections ConnectionBook
	Pushes      ConnectionGateway
	Presence    PresenceKeeper
}

func (sweep DomainSweep) Sweep() (SweepReport, error) {
	return sweep.sweepAll(sweep.Presence.Now())
}

func (sweep DomainSweep) sweepAll(now time.Time) (SweepReport, error) {
	report := SweepReport{}
	cursor := ""
	for page := 0; sweep.morePages(page, cursor, report); page++ {
		domains, next, err := sweep.Book.List(cursor, sweep.domainPageSize())
		if err != nil {
			return report, err
		}
		report, err = sweep.sweepDomains(domains, report, now)
		if err != nil {
			return report, err
		}
		cursor = next
	}
	return report, nil
}

func (sweep DomainSweep) morePages(page int, cursor string, report SweepReport) bool {
	return sweep.Remaining(report) > 0 && (page == 0 || cursor != "")
}

func (sweep DomainSweep) sweepDomains(domains []string, report SweepReport, now time.Time) (SweepReport, error) {
	for index := 0; index < len(domains) && sweep.Remaining(report) > 0; index++ {
		swept, err := sweep.sweepDomain(domains[index], report, now)
		if err != nil {
			return report, err
		}
		report = swept
	}
	return report, nil
}

func (sweep DomainSweep) sweepDomain(domainId string, report SweepReport, now time.Time) (SweepReport, error) {
	nowText := TimeText{}.Format(now)
	expired, err := sweep.expired(domainId, nowText, sweep.Remaining(report)/sweep.writesPerClose())
	if err != nil {
		return report, err
	}
	closed, err := sweep.closeAll(expired)
	report = SweepReports{}.Sum(report, closed)
	if err != nil {
		return report, err
	}
	settled, err := sweep.Presence.Settle(domainId, nowText, sweep.Remaining(report))
	return SweepReports{}.Sum(report, settled), err
}

func (sweep DomainSweep) expired(domainId string, now string, limit int) ([]Connection, error) {
	if limit <= 0 {
		return []Connection{}, nil
	}
	return sweep.Connections.Expired(domainId, now, limit)
}

func (sweep DomainSweep) closeAll(connections []Connection) (SweepReport, error) {
	report := SweepReport{}
	for _, connection := range connections {
		err := sweep.closeOne(connection)
		if err != nil {
			return report, err
		}
		report = SweepReports{}.Sum(report, SweepReport{ClosedConnections: 1, Writes: sweep.writesPerClose()})
	}
	return report, nil
}

func (sweep DomainSweep) closeOne(connection Connection) error {
	err := sweep.ignoreGone(sweep.Pushes.Push(connection.ConnectionId, Event{Type: sweep.expiredType(), State: sweep.expiredState()}))
	if err != nil {
		return err
	}
	err = sweep.ignoreGone(sweep.Pushes.Close(connection.ConnectionId, sweep.expiredCode()))
	if err != nil {
		return err
	}
	_, err = sweep.Connections.Remove(connection.DomainId, connection.ConnectionId)
	if err != nil {
		return err
	}
	return sweep.Presence.Disconnected(connection.DomainId, connection.MemberId)
}

func (DomainSweep) ignoreGone(err error) error {
	if errors.As(err, &Gone{}) {
		return nil
	}
	return err
}

func (sweep DomainSweep) Remaining(report SweepReport) int {
	return sweep.budget() - report.Writes
}

func (DomainSweep) budget() int {
	return 25
}

func (DomainSweep) writesPerClose() int {
	return 2
}

func (DomainSweep) domainPageSize() int {
	return 100
}

func (DomainSweep) expiredType() string {
	return "auth.expired"
}

func (DomainSweep) expiredState() string {
	return "4401"
}

func (DomainSweep) expiredCode() int {
	return 4401
}
