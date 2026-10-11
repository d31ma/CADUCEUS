// DeliveryRefusal is an expected failure a delivery route answers with its
// status (400, 404 or 409) and message, as the former ConversationArchive,
// Snooze and SendIdempotency errors were. Kind names an idempotency conflict
// (mismatch, applying or ambiguous).
type DeliveryRefusal struct {
	Message string
	Status  int
	Kind    string
}

func (refusal *DeliveryRefusal) Error() string {
	return refusal.Message
}

// Answer answers the refusal as its route answer.
func (refusal *DeliveryRefusal) Answer() map[string]any {
	switch refusal.Status {
	case 404:
		return map[string]any{"error": refusal.Message, "notFound": true}
	case 409:
		return map[string]any{"error": refusal.Message, "conflict": true}
	}
	return map[string]any{"error": refusal.Message}
}
