package realtime

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type Sweeper struct {
	Gate    oidc.Gate
	Domains DomainSweep
	Tickets TicketSweep
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
	report, err := sweeper.Domains.Sweep()
	if err != nil {
		return report, err
	}
	cleaned, err := sweeper.Tickets.Clean(sweeper.Domains.Remaining(report))
	report = SweepReports{}.Sum(report, cleaned)
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

func (sweeper Sweeper) more(report SweepReport) bool {
	return report.More || sweeper.Domains.Remaining(report) <= 0
}

func (Sweeper) operatorsRealm() string {
	return "mesail-operators"
}

func (Sweeper) serviceRole() string {
	return "Service"
}
