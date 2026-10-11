import "strings"

// PresenceRegistry keeps who is online, one FYLO document per user, so every
// worker sees the same presence. FYLO's putData always makes a new document,
// so a heartbeat deletes the user's earlier record first.
type PresenceRegistry struct {
	Store FyloCollection
}

// Heartbeat marks the user online in the given domains.
func (registry PresenceRegistry) Heartbeat(email string, domains []string) error {
	if email == "" {
		return nil
	}
	normalized := strings.ToLower(email)
	previous, _, err := registry.find(normalized)
	if err != nil {
		return err
	}
	if previous != "" {
		if err := registry.Store.Delete(previous); err != nil {
			return err
		}
	}
	if domains == nil {
		domains = []string{}
	}
	_, err = registry.Store.Put(map[string]any{"email": normalized, "domains": domains, "lastSeenAt": EpochTime{}.Now()})
	return err
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
	for _, document := range documents {
		record := document.Data
		lastSeen := RecordValues{}.Numeric(record["lastSeenAt"])
		if now-lastSeen > (PresenceRules{}).PruneAfter() {
			_ = registry.Store.Delete(document.Id)
			continue
		}
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

func (registry PresenceRegistry) find(email string) (string, map[string]any, error) {
	record, err := registry.Store.First("email", email)
	return record.Id, record.Data, err
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
