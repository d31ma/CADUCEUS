package realtime

import "time"

type PresenceDue struct {
	Now time.Time
}

func (rule PresenceDue) Due(record PresenceRecord) bool {
	return rule.due(record.State != PresenceStates{}.Offline(), rule.waitedOut(record), rule.ended(record))
}

func (rule PresenceDue) Ended(record PresenceRecord) bool {
	return rule.ended(record)
}

func (PresenceDue) due(online bool, waitedOut bool, ended bool) bool {
	return online && (waitedOut || ended)
}

func (rule PresenceDue) waitedOut(record PresenceRecord) bool {
	disconnectedAt := TimeText{}.At(record.DisconnectedAt)
	return !disconnectedAt.IsZero() && !disconnectedAt.After(rule.Now.Add(-rule.grace()))
}

func (rule PresenceDue) ended(record PresenceRecord) bool {
	until := TimeText{}.At(record.DoNotDisturbUntil)
	return record.State == PresenceStates{}.DoNotDisturb() && !until.IsZero() && !until.After(rule.Now)
}

func (PresenceDue) grace() time.Duration {
	return 60 * time.Second
}
