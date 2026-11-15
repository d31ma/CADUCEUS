package syncing

import (
	"errors"
	"slices"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type changeRows struct {
	changes core.ChangeStore
	now     func() time.Time
}

func (rows changeRows) ensured(envelope events.Envelope, entries []changeEntry) ([]core.Change, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	existing, err := rows.ofEvent(envelope)
	if err != nil {
		return nil, err
	}
	added, err := rows.appended(envelope, rows.missing(entries, existing))
	return append(existing, added...), err
}

func (rows changeRows) ofEvent(envelope events.Envelope) ([]core.Change, error) {
	if rows.fresh(envelope) {
		return nil, nil
	}
	return rows.storedOfEvent(envelope)
}

func (rows changeRows) fresh(envelope events.Envelope) bool {
	occurred, err := time.Parse(time.RFC3339Nano, envelope.OccurredAt)
	if err != nil {
		return false
	}
	return rows.now().Sub(occurred) < rows.freshFor()
}

func (rows changeRows) storedOfEvent(envelope events.Envelope) ([]core.Change, error) {
	found := []core.Change{}
	failures := []error{}
	for _, month := range rows.months(envelope) {
		changes, err := rows.changes.OfEvent(envelope.DomainId, month, envelope.EventID)
		found, failures = append(found, changes...), append(failures, err)
	}
	return found, rows.unavailable(errors.Join(failures...))
}

func (rows changeRows) months(envelope events.Envelope) []string {
	ids := core.MessageIds{}
	occurred, err := time.Parse(time.RFC3339Nano, envelope.OccurredAt)
	current := ids.Month(rows.now())
	if err != nil {
		return []string{current}
	}
	return slices.Compact([]string{ids.Month(occurred), current})
}

func (changeRows) missing(entries []changeEntry, existing []core.Change) []changeEntry {
	return slices.DeleteFunc(slices.Clone(entries), func(entry changeEntry) bool {
		return slices.ContainsFunc(existing, func(change core.Change) bool {
			return change.MessageId == entry.messageId
		})
	})
}

func (rows changeRows) appended(envelope events.Envelope, entries []changeEntry) ([]core.Change, error) {
	added := []core.Change{}
	for _, entry := range entries {
		change := rows.row(envelope, entry)
		id, err := rows.changes.Append(change)
		if err != nil {
			return added, rows.unavailable(err)
		}
		change.Id = id
		added = append(added, change)
	}
	return added, nil
}

func (rows changeRows) row(envelope events.Envelope, entry changeEntry) core.Change {
	at := rows.now()
	return core.Change{
		DomainId:       envelope.DomainId,
		ConversationId: entry.conversationId,
		MessageId:      entry.messageId,
		EventId:        envelope.EventID,
		Month:          core.MessageIds{}.Month(at),
		LoggedAt:       core.ChangeCursors{}.Stamp(at),
	}
}

func (changeRows) freshFor() time.Duration {
	return 10 * time.Second
}

func (changeRows) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return core.Unavailable{}.Store(failure)
}
