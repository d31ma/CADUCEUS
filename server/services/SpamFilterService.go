// SpamFilterService classifies inbound mail as spam; see SpamFilter.
// @Service
type SpamFilterService struct{}

// Classified answers {isSpam, score, reasons} for a message's signals.
func (*SpamFilterService) Classified(signals map[string]any) map[string]any {
	return SpamFilter{}.Classified(signals)
}
