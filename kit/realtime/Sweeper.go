package realtime

import (
	"errors"
	"slices"
	"time"

	"mesail.dev/packages/oidc"
)

type Sweeper struct {
	Gate        oidc.Gate
	Domains     DomainBook
	Connections ConnectionBook
	Pushes      ConnectionGateway
	Presence    PresenceKeeper
}

func (sweeper Sweeper) Run(authorization string) (SweepReport, error) {
	caller, err := CallerGate{Gate: sweeper.Gate}.Caller(authorization)
	if err != nil {
		return SweepReport{}, err
	}
	err = sweeper.requireOperators(caller)
	if err != nil {
		return SweepReport{}, err
	}
	err = sweeper.requireService(caller)
	if err != nil {
		return SweepReport{}, err
	}
	report, err := sweeper.sweepAll(sweeper.Presence.Now())
	report.More = sweeper.more(report)
	return report, err
}

func (sweeper Sweeper) requireOperators(caller oidc.Caller) error {
	if caller.DomainId != sweeper.operatorsRealm() {
		return Missing{What: "domain"}
	}
	return nil
}

func (sweeper Sweeper) requireService(caller oidc.Caller) error {
	if !slices.Contains(caller.Roles, sweeper.serviceRole()) {
		return Forbidden{}
	}
	return nil
}

func (sweeper Sweeper) sweepAll(now time.Time) (SweepReport, error) {
	report := SweepReport{}
	cursor := ""
	for page := 0; sweeper.morePages(page, cursor, report); page++ {
		domains, next, err := sweeper.Domains.List(cursor, sweeper.domainPageSize())
		if err != nil {
			return report, err
		}
		report, err = sweeper.sweepDomains(domains, report, now)
		if err != nil {
			return report, err
		}
		cursor = next
	}
	return report, nil
}

func (sweeper Sweeper) morePages(page int, cursor string, report SweepReport) bool {
	return sweeper.remaining(report) > 0 && (page == 0 || cursor != "")
}

func (sweeper Sweeper) sweepDomains(domains []string, report SweepReport, now time.Time) (SweepReport, error) {
	for index := 0; index < len(domains) && sweeper.remaining(report) > 0; index++ {
		swept, err := sweeper.sweepDomain(domains[index], report, now)
		if err != nil {
			return report, err
		}
		report = swept
	}
	return report, nil
}

func (sweeper Sweeper) more(report SweepReport) bool {
	return report.More || sweeper.remaining(report) <= 0
}

func (sweeper Sweeper) sweepDomain(domainId string, report SweepReport, now time.Time) (SweepReport, error) {
	nowText := TimeText{}.Format(now)
	expired, err := sweeper.expired(domainId, nowText, sweeper.remaining(report)/sweeper.writesPerClose())
	if err != nil {
		return report, err
	}
	closed, err := sweeper.closeAll(expired)
	report = SweepReports{}.Sum(report, closed)
	if err != nil {
		return report, err
	}
	settled, err := sweeper.Presence.Settle(domainId, nowText, sweeper.remaining(report))
	return SweepReports{}.Sum(report, settled), err
}

func (sweeper Sweeper) expired(domainId string, now string, limit int) ([]Connection, error) {
	if limit <= 0 {
		return []Connection{}, nil
	}
	return sweeper.Connections.Expired(domainId, now, limit)
}

func (sweeper Sweeper) closeAll(connections []Connection) (SweepReport, error) {
	report := SweepReport{}
	for _, connection := range connections {
		err := sweeper.closeOne(connection)
		if err != nil {
			return report, err
		}
		report = SweepReports{}.Sum(report, SweepReport{ClosedConnections: 1, Writes: sweeper.writesPerClose()})
	}
	return report, nil
}

func (sweeper Sweeper) closeOne(connection Connection) error {
	err := sweeper.ignoreGone(sweeper.Pushes.Push(connection.ConnectionId, Event{Type: sweeper.expiredType(), State: sweeper.expiredState()}))
	if err != nil {
		return err
	}
	err = sweeper.ignoreGone(sweeper.Pushes.Close(connection.ConnectionId, sweeper.expiredCode()))
	if err != nil {
		return err
	}
	_, err = sweeper.Connections.Remove(connection.DomainId, connection.ConnectionId)
	if err != nil {
		return err
	}
	return sweeper.Presence.Disconnected(connection.DomainId, connection.MemberId)
}

func (Sweeper) ignoreGone(err error) error {
	if errors.As(err, &Gone{}) {
		return nil
	}
	return err
}

func (sweeper Sweeper) remaining(report SweepReport) int {
	return sweeper.budget() - report.Writes
}

func (Sweeper) budget() int {
	return 25
}

func (Sweeper) writesPerClose() int {
	return 2
}

func (Sweeper) domainPageSize() int {
	return 100
}

func (Sweeper) operatorsRealm() string {
	return "mesail-operators"
}

func (Sweeper) serviceRole() string {
	return "Service"
}

func (Sweeper) expiredType() string {
	return "auth.expired"
}

func (Sweeper) expiredState() string {
	return "4401"
}

func (Sweeper) expiredCode() int {
	return 4401
}
