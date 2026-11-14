package devicebox

import (
	"maps"
	"slices"
	"time"
)

type ExpiryRun struct {
	parts    ExpiryParts
	now      time.Time
	deadline time.Time
	writes   int
	more     bool
}

func NewExpiryRun(parts ExpiryParts, now time.Time) *ExpiryRun {
	return &ExpiryRun{parts: parts, now: now, deadline: time.Now().Add(Limits{}.TimeBudget())}
}

func (run *ExpiryRun) Domains(domains []string) (ExpireReport, error) {
	for _, domainId := range domains {
		err := run.domain(domainId)
		if err != nil {
			return run.report(), err
		}
	}
	return run.report(), nil
}

func (run *ExpiryRun) domain(domainId string) error {
	slips, err := run.parts.Store.Expired(domainId, run.now.UnixMicro(), run.pageSize())
	if err != nil {
		return err
	}
	deltas := map[string]int{}
	finished, err := run.expireSlips(domainId, slips, deltas)
	writes, countError := run.parts.Counts.Add(domainId, deltas)
	run.writes += writes
	if err != nil || countError != nil {
		return run.firstError(err, countError)
	}
	return run.afterSlips(domainId, finished && len(slips) < run.pageSize())
}

func (run *ExpiryRun) afterSlips(domainId string, allSlipsDone bool) error {
	if !allSlipsDone {
		run.more = true
		return nil
	}
	return run.expireBodies(domainId)
}

func (run *ExpiryRun) expireSlips(domainId string, slips []Slip, deltas map[string]int) (bool, error) {
	for _, slip := range slips {
		if !run.affords(run.slipCost(slip, deltas)) {
			return false, nil
		}
		err := run.expireOne(domainId, slip, deltas)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (run *ExpiryRun) expireOne(domainId string, slip Slip, deltas map[string]int) error {
	err := run.parts.Store.DeleteEnvelope(domainId, slip)
	run.writes += run.deleteCost()
	if err != nil {
		return err
	}
	for deviceKey := range slip.Waiting {
		deltas[deviceKey]--
	}
	return run.noticeUnlessNotice(domainId, slip, deltas)
}

func (run *ExpiryRun) noticeUnlessNotice(domainId string, slip Slip, deltas map[string]int) error {
	if slip.Kind == (Formats{}).Undelivered() {
		return nil
	}
	written, err := NewUndeliveredNotice(run.parts, run.now).Leave(domainId, slip)
	run.writes += written
	deltas[DeviceKey{}.Of(slip.SenderDeviceId)]++
	return err
}

func (run *ExpiryRun) expireBodies(domainId string) error {
	room := Limits{}.WriteBudget() - run.writes
	if room < 1 {
		run.more = true
		return nil
	}
	bodies, err := run.parts.Store.ExpiredBodies(domainId, run.now.UnixMicro(), room)
	if err != nil {
		return err
	}
	run.more = run.more || len(bodies) == room
	return run.deleteBodies(domainId, bodies)
}

func (run *ExpiryRun) deleteBodies(domainId string, bodies []Body) error {
	for _, body := range bodies {
		err := run.parts.Store.DeleteBody(domainId, body.Id)
		run.writes++
		if err != nil {
			return err
		}
	}
	return nil
}

func (run *ExpiryRun) slipCost(slip Slip, deltas map[string]int) int {
	touched := slices.Collect(maps.Keys(deltas))
	touched = append(touched, slices.Collect(maps.Keys(slip.Waiting))...)
	touched = append(touched, DeviceKey{}.Of(slip.SenderDeviceId))
	return run.deleteCost() + run.noticeCost(slip) + run.parts.Counts.Buckets(touched)
}

func (run *ExpiryRun) noticeCost(slip Slip) int {
	if slip.Kind == (Formats{}).Undelivered() {
		return 0
	}
	return UndeliveredNotice{}.Cost()
}

func (run *ExpiryRun) affords(cost int) bool {
	return run.writes+cost <= Limits{}.WriteBudget() && time.Now().Before(run.deadline)
}

func (ExpiryRun) firstError(first error, second error) error {
	if first != nil {
		return first
	}
	return second
}

func (run *ExpiryRun) report() ExpireReport {
	return ExpireReport{Writes: run.writes, More: run.more}
}

func (ExpiryRun) deleteCost() int {
	return 2
}

func (ExpiryRun) pageSize() int {
	return 10
}
