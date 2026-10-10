// PresenceRules holds the presence windows. A user is online while their
// last heartbeat is within OnlineWindow; records older than PruneAfter are
// deleted lazily when a snapshot reads them.
type PresenceRules struct{}

// OnlineWindow is how long a heartbeat keeps a user online, in milliseconds.
func (PresenceRules) OnlineWindow() float64 { return 60_000 }

// PruneAfter is when a presence record is stale, in milliseconds.
func (PresenceRules) PruneAfter() float64 { return 10 * 60_000 }
