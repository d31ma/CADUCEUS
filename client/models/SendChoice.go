// SendChoice is how one message leaves the composer: sent now, sent and
// archived, scheduled, or a scheduled message moved.
type SendChoice struct {
	Mode           string
	SendAt         string
	DelayMs        int
	IdempotencyKey string
	RichText       bool
}
