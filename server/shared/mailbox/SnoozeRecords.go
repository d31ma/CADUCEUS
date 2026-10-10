import (
	"sort"
	"strings"
)

// SnoozeRecords selects snooze entries: the active ones of a mailbox, the
// ones due, and the ones for a thread or a client request.
type SnoozeRecords struct{}

// Active answers the active entries (no status, or "active") the owner may
// see; an empty owner, or an entry without one, matches every mailbox.
func (SnoozeRecords) Active(entries []FyloRecord, ownerEmail string) []FyloRecord {
	text := TextValue{}
	owner := strings.ToLower(ownerEmail)
	active := []FyloRecord{}
	for _, entry := range entries {
		status := entry.Data["status"]
		if text.Truthy(status) && status != "active" {
			continue
		}
		if owner == "" || !text.Truthy(entry.Data["ownerEmail"]) || strings.ToLower(text.Of(entry.Data["ownerEmail"])) == owner {
			active = append(active, entry)
		}
	}
	return active
}

// Due answers the entries due by now (milliseconds), soonest first, at most
// limit of them (from 1 to 500).
func (SnoozeRecords) Due(entries []FyloRecord, now int64, limit int) []FyloRecord {
	due := []FyloRecord{}
	for _, entry := range entries {
		if until, isNumber := entry.Data["until"].(float64); isNumber && until <= float64(now) {
			due = append(due, entry)
		}
	}
	sort.SliceStable(due, func(left, right int) bool {
		return due[left].Data["until"].(float64) < due[right].Data["until"].(float64)
	})
	return due[:min(len(due), max(1, min(500, limit)))]
}

// ForThread answers the first entry of a thread, or an empty record.
func (SnoozeRecords) ForThread(entries []FyloRecord, threadId string) FyloRecord {
	for _, entry := range entries {
		if entry.Data["threadId"] == threadId {
			return entry
		}
	}
	return FyloRecord{}
}

// ForClientRequest answers the owner's entry made by one client request, or
// an empty record.
func (SnoozeRecords) ForClientRequest(entries []FyloRecord, ownerEmail string, clientRequestId string) FyloRecord {
	owner := strings.ToLower(ownerEmail)
	for _, entry := range entries {
		stored, isText := entry.Data["ownerEmail"].(string)
		if isText && strings.ToLower(stored) == owner && entry.Data["clientRequestId"] == clientRequestId {
			return entry
		}
	}
	return FyloRecord{}
}
