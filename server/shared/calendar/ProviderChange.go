// ProviderChange is one edit of a provider event: the event's id, the
// fields it should have, the occurrence it changes ("" for the whole event)
// and the idempotency key of the mutation.
type ProviderChange struct {
	EventId        string
	Event          map[string]any
	RecurrenceId   string
	IdempotencyKey string
}
