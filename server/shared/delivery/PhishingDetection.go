import "regexp"

// PhishingDetection derives safe, displayable sender metadata from
// untrusted RFC headers, and the Reply-To mismatch warning.
type PhishingDetection struct{}

// Analyzed answers {headerFrom, replyTo, securityWarnings}.
func (detection PhishingDetection) Analyzed(envelopeSender string, headers map[string]any) map[string]any {
	headerFrom := Security{}.NormalizeEmailAddress(envelopeSender)
	if from := detection.addresses(headers["from"]); len(from) > 0 {
		headerFrom = from[0]
	}
	replyTo := detection.addresses(headers["reply-to"])
	warnings := []string{}
	for _, address := range replyTo {
		if address != headerFrom {
			warnings = []string{"reply_to_mismatch"}
		}
	}
	return map[string]any{"headerFrom": headerFrom, "replyTo": replyTo, "securityWarnings": warnings}
}

// addresses answers up to ten normalized addresses of a header, without
// trusting display names.
func (PhishingDetection) addresses(header any) []string {
	text, _ := header.(string)
	pattern := regexp.MustCompile("(?i)[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+")
	addresses := []string{}
	for _, match := range pattern.FindAllString(text, -1) {
		normalized := Security{}.NormalizeEmailAddress(match)
		if normalized != "" && !(TextValue{}).Includes(addresses, normalized) {
			addresses = append(addresses, normalized)
		}
		if len(addresses) == 10 {
			break
		}
	}
	return addresses
}
