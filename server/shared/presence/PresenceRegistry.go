import "strings"

// PresenceRegistry keeps who is online, one FYLO document per user, so every
// worker sees the same presence. FYLO's putData always makes a new document,
// so a heartbeat writes its record first and then removes the user's older
// ones. Two heartbeats at the same moment therefore never leave the user
// without a record: each removes only records older than its own, and the
// newest survives.
type PresenceRegistry struct {
	Store FyloCollection
}

// Heartbeat marks the user online in the given domains.
func (registry PresenceRegistry) Heartbeat(email string, domains []string) error {
	if email == "" {
		return nil
	}
	normalized := strings.ToLower(email)
	if domains == nil {
		domains = []string{}
	}
	seenAt := EpochTime{}.Now()
	id, err := registry.Store.Put(map[string]any{"email": normalized, "domains": domains, "lastSeenAt": seenAt})
	if err != nil {
		return err
	}
	records, err := registry.Store.Where("email", normalized)
	if err != nil {
		return err
	}
	for _, record := range records {
		if registry.older(record, id, float64(seenAt)) {
			// A concurrent heartbeat may have removed it already.
			_ = registry.Store.Delete(record.Id)
		}
	}
	return nil
}

// older reports whether a stored record predates the one just written:
// earlier lastSeenAt, or the same instant and a smaller document id.
func (registry PresenceRegistry) older(record FyloRecord, id string, seenAt float64) bool {
	if record.Id == id {
		return false
	}
	lastSeen := RecordValues{}.Numeric(record.Data["lastSeenAt"])
	return lastSeen < seenAt || (lastSeen == seenAt && record.Id < id)
}

// Snapshot answers every recently seen user who shares a domain with the
// viewer, as {email, online, lastSeenAt}. Stale records are deleted; a failed
// delete is ignored, as the former background delete was.
func (registry PresenceRegistry) Snapshot(viewerDomains []string) ([]map[string]any, error) {
	entries := []map[string]any{}
	if len(viewerDomains) == 0 {
		return entries, nil
	}
	viewer := map[string]bool{}
	for _, domain := range viewerDomains {
		viewer[strings.ToLower(domain)] = true
	}
	documents, err := registry.Store.All()
	if err != nil {
		return nil, err
	}
	now := float64(EpochTime{}.Now())
	latest := map[string]float64{}
	for _, document := range documents {
		email := strings.ToLower(TextValue{}.Of(document.Data["email"]))
		latest[email] = max(latest[email], RecordValues{}.Numeric(document.Data["lastSeenAt"]))
	}
	listed := map[string]bool{}
	for _, document := range documents {
		record := document.Data
		lastSeen := RecordValues{}.Numeric(record["lastSeenAt"])
		if now-lastSeen > (PresenceRules{}).PruneAfter() {
			_ = registry.Store.Delete(document.Id)
			continue
		}
		email := strings.ToLower(TextValue{}.Of(record["email"]))
		if lastSeen < latest[email] || listed[email] {
			continue
		}
		listed[email] = true
		if registry.shares(record["domains"], viewer) {
			entries = append(entries, map[string]any{
				"email":      record["email"],
				"online":     now-lastSeen < PresenceRules{}.OnlineWindow(),
				"lastSeenAt": EpochTime{}.Text(int64(lastSeen)),
			})
		}
	}
	return entries, nil
}

func (PresenceRegistry) shares(domains any, viewer map[string]bool) bool {
	list, _ := RecordValues{}.List(domains)
	for _, domain := range list {
		if text, isText := domain.(string); isText && viewer[strings.ToLower(text)] {
			return true
		}
	}
	return false
}
