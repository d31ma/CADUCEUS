import (
	"regexp"
	"strings"
)

// Threading normalises RFC 5322 Message-IDs and builds the thread metadata
// stored with each message.
type Threading struct{}

func (Threading) longestMessageId() int { return 998 }

func (Threading) mostReferences() int { return 100 }

// NormalizeMessageId answers one Message-ID in its stored form, always in
// angle brackets, or "" for control characters, several ids or anything
// that is not an id.
func (threading Threading) NormalizeMessageId(value any) string {
	raw, isText := value.(string)
	if !isText {
		return ""
	}
	text := TextValue{}
	input := text.Trim(raw)
	if input == "" || text.Length(input) > threading.longestMessageId()+2 || regexp.MustCompile(`[\r\n\x{0000}-\x{001f}\x{007f}]`).MatchString(input) {
		return ""
	}
	candidate := input
	if strings.HasPrefix(input, "<") && strings.HasSuffix(input, ">") {
		candidate = text.Trim(input[1 : len(input)-1])
	}
	if candidate == "" || text.Length(candidate) > threading.longestMessageId() || strings.ContainsAny(candidate, "<>") {
		return ""
	}
	at := strings.LastIndex(candidate, "@")
	if at <= 0 || at == len(candidate)-1 || strings.IndexFunc(candidate, text.Space) >= 0 {
		return ""
	}
	return "<" + candidate + ">"
}

// ParseMessageIdList answers the normalised, unique ids of a References-style
// value (a list, or text), at most 100 of them.
func (threading Threading) ParseMessageIdList(value any) []string {
	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case string:
		values = threading.candidates(typed)
	}
	normalized := []string{}
	seen := map[string]bool{}
	for _, candidate := range values {
		id := threading.NormalizeMessageId(candidate)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		normalized = append(normalized, id)
		if len(normalized) >= threading.mostReferences() {
			break
		}
	}
	return normalized
}

// ThreadMetadataFromHeaders answers the thread metadata of a received
// message from its lower-case MIME headers.
func (threading Threading) ThreadMetadataFromHeaders(headers map[string]any, fallbackId string, domain string) map[string]any {
	rfcMessageId := threading.first(threading.ParseMessageIdList(headers["message-id"]))
	if rfcMessageId == "" {
		rfcMessageId = threading.CreateMessageId(domain, fallbackId)
	}
	inReplyTo := threading.first(threading.ParseMessageIdList(headers["in-reply-to"]))
	return threading.metadata(domain, rfcMessageId, inReplyTo, threading.ParseMessageIdList(headers["references"]))
}

// ThreadMetadataFromOutbound answers the thread metadata of a message
// composed here (its messageId, inReplyTo and references fields).
func (threading Threading) ThreadMetadataFromOutbound(message map[string]any, fallbackId string, domain string) map[string]any {
	rfcMessageId := threading.NormalizeMessageId(message["messageId"])
	if rfcMessageId == "" {
		rfcMessageId = threading.CreateMessageId(domain, fallbackId)
	}
	inReplyTo := threading.NormalizeMessageId(message["inReplyTo"])
	return threading.metadata(domain, rfcMessageId, inReplyTo, threading.ParseMessageIdList(message["references"]))
}

// CreateMessageId answers an unguessable, standards-shaped Message-ID for
// local delivery.
func (Threading) CreateMessageId(domain string, hint string) string {
	random := RandomText{}
	if domain == "" {
		domain = "localhost"
	}
	safeDomain := regexp.MustCompile(`[^a-z0-9.-]`).ReplaceAllString(strings.ToLower(domain), "")
	if safeDomain == "" {
		safeDomain = "localhost"
	}
	token := regexp.MustCompile(`[^a-z0-9._-]`).ReplaceAllString(strings.ToLower(hint), "")
	if len(token) > 80 {
		token = token[:80]
	}
	if token == "" {
		token = random.Hex(16)
	}
	return "<" + token + "." + random.Hex(8) + "@" + safeDomain + ">"
}

// DeriveThreadId answers the tenant-scoped thread id of a conversation root.
func (Threading) DeriveThreadId(domain string, root string) string {
	return RandomText{}.Digest([]byte(strings.ToLower(domain) + "\x00" + root))
}

func (threading Threading) metadata(domain string, rfcMessageId string, inReplyTo string, references []string) map[string]any {
	lineage := append([]string{}, references...)
	if inReplyTo != "" && !(TextValue{}).Includes(lineage, inReplyTo) {
		lineage = append(lineage, inReplyTo)
	}
	root := rfcMessageId
	if len(lineage) > 0 {
		root = lineage[0]
	}
	metadata := map[string]any{"rfcMessageId": rfcMessageId, "threadId": threading.DeriveThreadId(domain, root)}
	if inReplyTo != "" {
		metadata["inReplyTo"] = inReplyTo
	}
	if len(lineage) > 0 {
		metadata["references"] = lineage
	}
	return metadata
}

func (Threading) first(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func (Threading) candidates(value string) []any {
	candidates := []any{}
	for _, match := range regexp.MustCompile(`<([^<>\r\n]+)>`).FindAllStringSubmatch(value, -1) {
		candidates = append(candidates, "<"+match[1]+">")
	}
	if len(candidates) > 0 {
		return candidates
	}
	for _, part := range strings.FieldsFunc(value, func(letter rune) bool { return letter == ',' || (TextValue{}).Space(letter) }) {
		candidates = append(candidates, part)
	}
	return candidates
}
