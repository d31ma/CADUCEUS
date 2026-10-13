import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// MessageConversationUtils splits plain-text quotations and searches the
// visible fields of a conversation, as message-conversation.js did.
type MessageConversationUtils struct{}

func (MessageConversationUtils) newline() string { return "\n" }

func (MessageConversationUtils) SplitQuotedText(value string) QuotedText {
	newline := MessageConversationUtils{}.newline()
	carriage := "\r"
	text := strings.ReplaceAll(strings.ReplaceAll(value, carriage+newline, newline), carriage, newline)
	lines := strings.Split(text, newline)
	index := MessageConversationUtils{}.quoteStart(lines)
	return MessageConversationUtils{}.splitAt(text, lines, index)
}

func (MessageConversationUtils) HasText(parts QuotedText) bool {
	return MessageConversationUtils{}.hasText(parts)
}

func (MessageConversationUtils) hasText(parts QuotedText) bool {
	return parts.Visible != "" || parts.HasQuoted
}

func (MessageConversationUtils) IsForwardedSubject(value string) bool {
	return MessageConversationUtils{}.forwardedSubject().MatchString(value)
}

func (MessageConversationUtils) SearchConversation(messages []ConversationMessage, rawTerm string, requestedIndex int) ConversationSearch {
	term := MessageConversationUtils{}.searchTerm(rawTerm)
	progress := &SearchProgress{Results: []FindResult{}}
	decorated := MessageConversationUtils{}.decorateAll(messages, term, progress)
	index := MessageConversationUtils{}.currentIndex(requestedIndex, len(progress.Results))
	return ConversationSearch{
		Term:     term,
		Messages: MessageConversationUtils{}.markAll(decorated, index),
		Results:  progress.Results,
		Index:    index,
		Count:    len(progress.Results),
	}
}

func (MessageConversationUtils) quotePatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`^\s*>`),
		regexp.MustCompile(`(?i)^\s*On .{1,240} wrote:\s*$`),
		regexp.MustCompile(`(?i)^\s*-{2,}\s*(?:Original|Forwarded) Message\s*-{2,}\s*$`),
		regexp.MustCompile(`(?i)^\s*Begin forwarded message:\s*$`),
		regexp.MustCompile(`^\s*_{5,}\s*$`),
	}
}

func (MessageConversationUtils) forwardedSubject() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^\s*fwd?\s*:`)
}

func (MessageConversationUtils) quoteStart(lines []string) int {
	patterns := MessageConversationUtils{}.quotePatterns()
	return slices.IndexFunc(lines, func(line string) bool { return MessageConversationUtils{}.startsQuote(line, patterns) })
}

func (MessageConversationUtils) startsQuote(line string, patterns []*regexp.Regexp) bool {
	return slices.ContainsFunc(patterns, func(pattern *regexp.Regexp) bool { return pattern.MatchString(line) })
}

func (MessageConversationUtils) splitAt(text string, lines []string, index int) QuotedText {
	if index < 0 {
		return QuotedText{Visible: text}
	}
	newline := MessageConversationUtils{}.newline()
	visible := strings.TrimRightFunc(strings.Join(lines[:index], newline), unicode.IsSpace)
	quoted := strings.TrimSpace(strings.Join(lines[index:], newline))
	return MessageConversationUtils{}.quotedParts(text, visible, quoted)
}

func (MessageConversationUtils) quotedParts(text string, visible string, quoted string) QuotedText {
	if quoted == "" {
		return QuotedText{Visible: text}
	}
	return QuotedText{Visible: visible, Quoted: quoted, HasQuoted: true}
}

func (MessageConversationUtils) maximumTermLength() int { return 200 }

func (MessageConversationUtils) searchTerm(rawTerm string) string {
	runes := []rune(strings.TrimSpace(rawTerm))
	return string(runes[:min(len(runes), MessageConversationUtils{}.maximumTermLength())])
}

func (MessageConversationUtils) currentIndex(requested int, count int) int {
	if count == 0 {
		return -1
	}
	return ((requested % count) + count) % count
}

func (MessageConversationUtils) decorateAll(messages []ConversationMessage, term string, progress *SearchProgress) []ConversationMessage {
	decorated := []ConversationMessage{}
	for _, message := range messages {
		decorated = append(decorated, MessageConversationUtils{}.decorate(message, term, progress))
	}
	return decorated
}

func (MessageConversationUtils) decorate(message ConversationMessage, term string, progress *SearchProgress) ConversationMessage {
	message.FindSenderSegments = MessageConversationUtils{}.fieldSegments(message.Sender, term, FindResult{MessageId: message.Id, Field: "sender"}, progress)
	message.FindRecipientSegments = MessageConversationUtils{}.fieldSegments(message.Recipient, term, FindResult{MessageId: message.Id, Field: "recipient"}, progress)
	message.FindSubjectSegments = MessageConversationUtils{}.fieldSegments(message.Subject, term, FindResult{MessageId: message.Id, Field: "subject"}, progress)
	message.FindBodySegments = MessageConversationUtils{}.fieldSegments(message.Body, term, FindResult{MessageId: message.Id, Field: "body"}, progress)
	return message
}

func (MessageConversationUtils) maximumMatches() int { return 1000 }

func (MessageConversationUtils) maximumFieldLength() int { return 1048576 }

func (MessageConversationUtils) fieldSegments(value string, term string, spot FindResult, progress *SearchProgress) []FindSegment {
	runes := []rune(value)
	text := runes[:min(len(runes), MessageConversationUtils{}.maximumFieldLength())]
	needle := MessageConversationUtils{}.lowered([]rune(term))
	positions := MessageConversationUtils{}.positions(MessageConversationUtils{}.lowered(text), needle)
	allowed := positions[:min(len(positions), MessageConversationUtils{}.maximumMatches()-progress.Next)]
	progress.Spot = spot
	return MessageConversationUtils{}.segments(text, len(needle), allowed, progress)
}

func (MessageConversationUtils) lowered(text []rune) []rune {
	lower := make([]rune, len(text))
	for index, character := range text {
		lower[index] = unicode.ToLower(character)
	}
	return lower
}

func (MessageConversationUtils) positions(haystack []rune, needle []rune) []int {
	text := string(haystack)
	offsets := MessageConversationUtils{}.runeOffsets(text)
	return MessageConversationUtils{}.translated(MessageConversationUtils{}.bytePositions(text, string(needle)), offsets)
}

func (MessageConversationUtils) bytePositions(haystack string, needle string) []int {
	found := []int{}
	for at := (MessageConversationUtils{}).indexFrom(haystack, needle, 0); at >= 0; at = (MessageConversationUtils{}).indexFrom(haystack, needle, at+max(len(needle), 1)) {
		found = append(found, at)
	}
	return found
}

func (MessageConversationUtils) indexFrom(haystack string, needle string, from int) int {
	if needle == "" || from > len(haystack) {
		return -1
	}
	return MessageConversationUtils{}.shifted(strings.Index(haystack[from:], needle), from)
}

func (MessageConversationUtils) shifted(index int, from int) int {
	if index < 0 {
		return -1
	}
	return index + from
}

func (MessageConversationUtils) runeOffsets(text string) map[int]int {
	offsets := map[int]int{}
	count := 0
	for byteIndex := range text {
		offsets[byteIndex] = count
		count++
	}
	return offsets
}

func (MessageConversationUtils) translated(bytePositions []int, offsets map[int]int) []int {
	found := []int{}
	for _, position := range bytePositions {
		found = append(found, offsets[position])
	}
	return found
}

func (MessageConversationUtils) segments(text []rune, length int, positions []int, progress *SearchProgress) []FindSegment {
	segments := []FindSegment{}
	cursor := 0
	for _, at := range positions {
		segments = append(segments, MessageConversationUtils{}.gap(text, cursor, at)...)
		segments = append(segments, MessageConversationUtils{}.match(text[at:at+length], progress))
		cursor = at + max(length, 1)
	}
	return append(segments, MessageConversationUtils{}.gap(text, cursor, len(text))...)
}

func (MessageConversationUtils) gap(text []rune, from int, to int) []FindSegment {
	if to <= from {
		return []FindSegment{}
	}
	return []FindSegment{{Text: string(text[from:to]), Index: -1}}
}

func (MessageConversationUtils) match(text []rune, progress *SearchProgress) FindSegment {
	spot := progress.Spot
	spot.Index = progress.Next
	progress.Next++
	progress.Results = append(progress.Results, spot)
	return FindSegment{Text: string(text), Match: true, Index: spot.Index}
}

func (MessageConversationUtils) markAll(messages []ConversationMessage, index int) []ConversationMessage {
	marked := []ConversationMessage{}
	for _, message := range messages {
		message.FindSenderSegments = MessageConversationUtils{}.markCurrent(message.FindSenderSegments, index)
		message.FindRecipientSegments = MessageConversationUtils{}.markCurrent(message.FindRecipientSegments, index)
		message.FindSubjectSegments = MessageConversationUtils{}.markCurrent(message.FindSubjectSegments, index)
		message.FindBodySegments = MessageConversationUtils{}.markCurrent(message.FindBodySegments, index)
		marked = append(marked, message)
	}
	return marked
}

func (MessageConversationUtils) markCurrent(segments []FindSegment, index int) []FindSegment {
	marked := []FindSegment{}
	for _, segment := range segments {
		segment.Current = segment.Index == index
		marked = append(marked, segment)
	}
	return marked
}
