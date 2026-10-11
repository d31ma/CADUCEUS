// HeaderAddresses reads the mailbox addresses of an untrusted header without
// trusting display names, and the Reply-To mismatch warning they imply.
type HeaderAddresses struct{}

// Found answers the first ten unique normalised addresses of a header.
func (HeaderAddresses) Found(header any) []string {
	text, isText := header.(string)
	if !isText || text == "" {
		return []string{}
	}
	addresses := []string{}
	for _, match := range (MailAddresses{}).Found(text) {
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

// Sender answers the header From address (the envelope sender when the
// header has none), the Reply-To addresses and the security warnings.
func (addresses HeaderAddresses) Sender(envelopeSender string, headers map[string]any) (string, []string, []string) {
	headerFrom := Security{}.NormalizeEmailAddress(envelopeSender)
	if found := addresses.Found(headers["from"]); len(found) > 0 {
		headerFrom = found[0]
	}
	replyTo := addresses.Found(headers["reply-to"])
	warnings := []string{}
	for _, address := range replyTo {
		if address != headerFrom {
			warnings = []string{"reply_to_mismatch"}
		}
	}
	return headerFrom, replyTo, warnings
}
