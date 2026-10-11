// MailActionInput is a validated POST /inbox/actions body: the message ids,
// the patch and the optional idempotency key.
type MailActionInput struct {
	Ids             []string
	Patch           map[string]any
	ClientRequestId string
}
