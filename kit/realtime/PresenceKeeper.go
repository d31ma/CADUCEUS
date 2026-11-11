package realtime

import (
	"time"

	"mesail.dev/packages/events"
)

type PresenceKeeper struct {
	Book        PresenceBook
	Connections ConnectionBook
	FanOut      FanOut
	Now         func() time.Time
}

func (keeper PresenceKeeper) Set(domainId string, memberId string, request PresenceRequest) error {
	err := PresenceRequestCheck{Now: keeper.Now()}.Check(request)
	if err != nil {
		return err
	}
	record, _, err := keeper.Book.Find(domainId, memberId)
	if err != nil {
		return err
	}
	changed := keeper.chosen(record, domainId, memberId, request)
	return keeper.save(domainId, changed, keeper.announced(record, changed))
}

func (keeper PresenceKeeper) Connected(domainId string, memberId string) error {
	record, found, err := keeper.Book.Find(domainId, memberId)
	if err != nil {
		return err
	}
	changed := keeper.connected(record, domainId, memberId, keeper.wakes(record, found))
	return keeper.saveWhenChanged(domainId, record, changed)
}

func (keeper PresenceKeeper) Disconnected(domainId string, memberId string) error {
	count, err := keeper.Connections.CountForMember(domainId, memberId)
	if err != nil {
		return err
	}
	record, found, err := keeper.Book.Find(domainId, memberId)
	if err != nil {
		return err
	}
	changed := keeper.disconnected(record, keeper.marksDisconnect(record, found, count))
	return keeper.saveWhenChanged(domainId, record, changed)
}

func (keeper PresenceKeeper) Query(domainId string, memberIds []string) ([]Presence, error) {
	unique, err := keeper.checkedIds(memberIds)
	if err != nil {
		return nil, err
	}
	records, err := keeper.Book.ForMembers(domainId, unique)
	if err != nil {
		return nil, err
	}
	err = keeper.requireSome(records)
	if err != nil {
		return nil, err
	}
	return keeper.views(domainId, records, PresenceDue{Now: keeper.Now()})
}

func (keeper PresenceKeeper) Settle(domainId string, now string, budget int) (SweepReport, error) {
	records, err := keeper.Book.Due(domainId, now, budget)
	if err != nil {
		return SweepReport{}, err
	}
	report, err := keeper.settleAll(domainId, records, PresenceDue{Now: TimeText{}.At(now)})
	report.More = len(records) >= budget
	return report, err
}

func (keeper PresenceKeeper) chosen(record PresenceRecord, domainId string, memberId string, request PresenceRequest) PresenceRecord {
	record.DomainId = domainId
	record.MemberId = memberId
	record.State = request.State
	record.DoNotDisturbUntil = keeper.endFor(request)
	record.LastSeenAt = TimeText{}.Format(keeper.Now())
	return record
}

func (PresenceKeeper) endFor(request PresenceRequest) string {
	if request.State == (PresenceStates{}).DoNotDisturb() {
		return request.DoNotDisturbUntil
	}
	return ""
}

func (PresenceKeeper) announced(before PresenceRecord, after PresenceRecord) bool {
	return before.State != after.State || before.DoNotDisturbUntil != after.DoNotDisturbUntil
}

func (PresenceKeeper) wakes(record PresenceRecord, found bool) bool {
	return !found || record.State == PresenceStates{}.Offline()
}

func (keeper PresenceKeeper) connected(record PresenceRecord, domainId string, memberId string, wakes bool) PresenceRecord {
	record.DisconnectedAt = ""
	if !wakes {
		return record
	}
	record.DomainId = domainId
	record.MemberId = memberId
	record.State = PresenceStates{}.Active()
	record.LastSeenAt = TimeText{}.Format(keeper.Now())
	return record
}

func (PresenceKeeper) marksDisconnect(record PresenceRecord, found bool, count int) bool {
	return count == 0 && found && record.State != PresenceStates{}.Offline() && record.DisconnectedAt == ""
}

func (keeper PresenceKeeper) disconnected(record PresenceRecord, marks bool) PresenceRecord {
	if marks {
		record.DisconnectedAt = TimeText{}.Format(keeper.Now())
	}
	return record
}

func (keeper PresenceKeeper) saveWhenChanged(domainId string, before PresenceRecord, after PresenceRecord) error {
	if before == after {
		return nil
	}
	return keeper.save(domainId, after, keeper.announced(before, after))
}

func (keeper PresenceKeeper) save(domainId string, record PresenceRecord, announce bool) error {
	err := keeper.Book.Save(record)
	if err != nil {
		return err
	}
	return keeper.announce(domainId, record, announce)
}

func (keeper PresenceKeeper) announce(domainId string, record PresenceRecord, announce bool) error {
	if !announce {
		return nil
	}
	return keeper.FanOut.ToDomain(domainId, Event{Type: events.Types{}.PresenceChanged(), MemberId: record.MemberId, State: record.State})
}

func (keeper PresenceKeeper) checkedIds(memberIds []string) ([]string, error) {
	unique := MemberSet{}.Without(memberIds, "")
	if len(unique) == 0 || len(unique) > keeper.maximumIds() {
		return nil, Invalid{Field: "memberIds", Reason: "must name 1 to 100 members"}
	}
	return unique, nil
}

func (PresenceKeeper) requireSome(records []PresenceRecord) error {
	if len(records) == 0 {
		return Missing{What: "presence"}
	}
	return nil
}

func (keeper PresenceKeeper) views(domainId string, records []PresenceRecord, rule PresenceDue) ([]Presence, error) {
	views := []Presence{}
	for _, record := range records {
		view, err := keeper.view(domainId, record, rule.Ended(record))
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (keeper PresenceKeeper) view(domainId string, record PresenceRecord, ended bool) (Presence, error) {
	count, err := keeper.countWhen(domainId, record.MemberId, ended)
	state := keeper.shownState(record.State, ended, count)
	return Presence{MemberId: record.MemberId, State: state, DoNotDisturbUntil: keeper.shownEnd(record, state), LastSeenAt: record.LastSeenAt}, err
}

func (keeper PresenceKeeper) countWhen(domainId string, memberId string, needed bool) (int, error) {
	if !needed {
		return 0, nil
	}
	return keeper.Connections.CountForMember(domainId, memberId)
}

func (PresenceKeeper) shownState(state string, ended bool, count int) string {
	states := PresenceStates{}
	switch {
	case !ended:
		return state
	case count > 0:
		return states.Active()
	}
	return states.Offline()
}

func (PresenceKeeper) shownEnd(record PresenceRecord, state string) string {
	if state == (PresenceStates{}).DoNotDisturb() {
		return record.DoNotDisturbUntil
	}
	return ""
}

func (keeper PresenceKeeper) settleAll(domainId string, records []PresenceRecord, rule PresenceDue) (SweepReport, error) {
	report := SweepReport{}
	for _, record := range records {
		settled, err := keeper.settleOne(domainId, record, rule.Ended(record))
		if err != nil {
			return report, err
		}
		report = SweepReports{}.Sum(report, settled)
	}
	return report, nil
}

func (keeper PresenceKeeper) settleOne(domainId string, record PresenceRecord, ended bool) (SweepReport, error) {
	count, err := keeper.Connections.CountForMember(domainId, record.MemberId)
	if err != nil {
		return SweepReport{}, err
	}
	settled := keeper.settled(record, ended, count)
	err = keeper.save(domainId, settled, record.State != settled.State)
	return keeper.tally(settled, ended), err
}

func (PresenceKeeper) settled(record PresenceRecord, ended bool, count int) PresenceRecord {
	states := PresenceStates{}
	record.DisconnectedAt = ""
	switch {
	case ended && count > 0:
		record.State = states.Active()
		record.DoNotDisturbUntil = ""
	case count == 0:
		record.State = states.Offline()
		record.DoNotDisturbUntil = ""
	}
	return record
}

func (PresenceKeeper) tally(settled PresenceRecord, ended bool) SweepReport {
	switch {
	case ended:
		return SweepReport{Writes: 1, EndedDoNotDisturb: 1}
	case settled.State == PresenceStates{}.Offline():
		return SweepReport{Writes: 1, OfflineMembers: 1}
	}
	return SweepReport{Writes: 1}
}

func (PresenceKeeper) maximumIds() int {
	return 100
}
