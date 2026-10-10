import (
	"html"
	"slices"
	"strings"
)

// HtmlTokenUtils reads the next HTML token from a parse state: text, a
// comment, a start tag with its attributes, or an end tag. Raw text elements
// such as style and script keep their content as one text token.
type HtmlTokenUtils struct{}

func (HtmlTokenUtils) Next(state *HtmlParseState) HtmlToken {
	return HtmlTokenUtils{}.tokenAt(state, state.Source[state.Position:])
}

func (HtmlTokenUtils) endOpen() string { return "</" }

func (HtmlTokenUtils) RawText(state *HtmlParseState, name string) string {
	rest := state.Source[state.Position:]
	return HtmlTokenUtils{}.consumeRaw(state, rest, strings.Index(strings.ToLower(rest), HtmlTokenUtils{}.endOpen()+name))
}

func (HtmlTokenUtils) letters() string {
	return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

func (HtmlTokenUtils) commentOpen() string { return "<!--" }

func (HtmlTokenUtils) tokenAt(state *HtmlParseState, rest string) HtmlToken {
	switch {
	case rest == "":
		return HtmlToken{}
	case strings.HasPrefix(rest, HtmlTokenUtils{}.commentOpen()):
		return HtmlTokenUtils{}.comment(state, rest[len(HtmlTokenUtils{}.commentOpen()):])
	case HtmlTokenUtils{}.isDeclaration(rest):
		return HtmlTokenUtils{}.declaration(state, rest)
	case HtmlTokenUtils{}.isEndTag(rest):
		return HtmlTokenUtils{}.endTag(state, rest)
	case HtmlTokenUtils{}.isStartTag(rest):
		return HtmlTokenUtils{}.startTag(state, rest)
	}
	return HtmlTokenUtils{}.text(state, rest)
}

func (HtmlTokenUtils) startsName(rest string) bool {
	return rest != "" && strings.IndexByte(HtmlTokenUtils{}.letters(), rest[0]) >= 0
}

func (HtmlTokenUtils) isDeclaration(rest string) bool {
	return strings.HasPrefix(rest, "<!") || strings.HasPrefix(rest, "<?")
}

func (HtmlTokenUtils) isEndTag(rest string) bool {
	return strings.HasPrefix(rest, HtmlTokenUtils{}.endOpen()) && HtmlTokenUtils{}.startsName(rest[len(HtmlTokenUtils{}.endOpen()):])
}

func (HtmlTokenUtils) tagOpen() string { return "<" }

func (HtmlTokenUtils) isStartTag(rest string) bool {
	return strings.HasPrefix(rest, HtmlTokenUtils{}.tagOpen()) && HtmlTokenUtils{}.startsName(rest[1:])
}

func (HtmlTokenUtils) commentClose() string { return "-->" }

func (HtmlTokenUtils) comment(state *HtmlParseState, body string) HtmlToken {
	end := strings.Index(body, HtmlTokenUtils{}.commentClose())
	return HtmlTokenUtils{}.commentToken(state, body, end)
}

func (HtmlTokenUtils) CommentKind() string { return "comment" }

func (HtmlTokenUtils) commentToken(state *HtmlParseState, body string, end int) HtmlToken {
	if end < 0 {
		state.Position = len(state.Source)
		return HtmlToken{Kind: HtmlTokenUtils{}.CommentKind(), Text: body}
	}
	state.Position += len(HtmlTokenUtils{}.commentOpen()) + end + len(HtmlTokenUtils{}.commentClose())
	return HtmlToken{Kind: HtmlTokenUtils{}.CommentKind(), Text: body[:end]}
}

func (HtmlTokenUtils) tagClose() byte { return '>' }

func (HtmlTokenUtils) declaration(state *HtmlParseState, rest string) HtmlToken {
	state.Position += HtmlTokenUtils{}.through(rest, strings.IndexByte(rest, HtmlTokenUtils{}.tagClose()))
	return HtmlToken{Kind: "declaration"}
}

func (HtmlTokenUtils) through(rest string, end int) int {
	if end < 0 {
		return len(rest)
	}
	return end + 1
}

func (HtmlTokenUtils) EndKind() string { return "end" }

func (HtmlTokenUtils) endTag(state *HtmlParseState, rest string) HtmlToken {
	name := HtmlTokenUtils{}.name(rest[len(HtmlTokenUtils{}.endOpen()):])
	state.Position += HtmlTokenUtils{}.through(rest, strings.IndexByte(rest, HtmlTokenUtils{}.tagClose()))
	return HtmlToken{Kind: HtmlTokenUtils{}.EndKind(), Name: name}
}

func (HtmlTokenUtils) spaces() string { return " \t\n\f\r" }

func (HtmlTokenUtils) name(text string) string {
	end := strings.IndexAny(text, HtmlTokenUtils{}.spaces()+"/>")
	return strings.ToLower(text[:HtmlTokenUtils{}.endOr(end, len(text))])
}

func (HtmlTokenUtils) endOr(end int, fallback int) int {
	if end < 0 {
		return fallback
	}
	return end
}

func (HtmlTokenUtils) StartKind() string { return "start" }

func (HtmlTokenUtils) startTag(state *HtmlParseState, rest string) HtmlToken {
	name := HtmlTokenUtils{}.name(rest[1:])
	token := HtmlToken{Kind: HtmlTokenUtils{}.StartKind(), Name: name, Attributes: []HtmlAttribute{}}
	state.Position += 1 + len(name)
	HtmlTokenUtils{}.attributes(state, &token)
	return token
}

func (HtmlTokenUtils) attributes(state *HtmlParseState, token *HtmlToken) {
	for HtmlTokenUtils{}.inTag(state) {
		HtmlTokenUtils{}.attributeStep(state, token)
	}
	state.Position = min(state.Position+1, len(state.Source))
}

func (HtmlTokenUtils) inTag(state *HtmlParseState) bool {
	return state.Position < len(state.Source) && state.Source[state.Position] != HtmlTokenUtils{}.tagClose()
}

func (HtmlTokenUtils) slash() byte { return '/' }

func (HtmlTokenUtils) attributeStep(state *HtmlParseState, token *HtmlToken) {
	character := state.Source[state.Position]
	switch {
	case strings.IndexByte(HtmlTokenUtils{}.spaces(), character) >= 0:
		state.Position++
	case character == HtmlTokenUtils{}.slash():
		state.Position++
		token.SelfClosing = HtmlTokenUtils{}.closesHere(state)
	default:
		HtmlTokenUtils{}.attribute(state, token)
	}
}

func (HtmlTokenUtils) closesHere(state *HtmlParseState) bool {
	return state.Position < len(state.Source) && state.Source[state.Position] == HtmlTokenUtils{}.tagClose()
}

func (HtmlTokenUtils) attribute(state *HtmlParseState, token *HtmlToken) {
	rest := state.Source[state.Position:]
	end := HtmlTokenUtils{}.endOr(strings.IndexAny(rest[1:], HtmlTokenUtils{}.spaces()+"/>="), len(rest)-1) + 1
	name := strings.ToLower(rest[:end])
	state.Position += end
	value := HtmlTokenUtils{}.attributeValue(state)
	token.Attributes = HtmlTokenUtils{}.withAttribute(token.Attributes, HtmlAttribute{Name: name, Value: value})
}

func (HtmlTokenUtils) attributeValue(state *HtmlParseState) string {
	rest := strings.TrimLeft(state.Source[state.Position:], HtmlTokenUtils{}.spaces())
	if !strings.HasPrefix(rest, "=") {
		return ""
	}
	value := strings.TrimLeft(rest[1:], HtmlTokenUtils{}.spaces())
	state.Position = len(state.Source) - len(value)
	return HtmlTokenUtils{}.value(state, value)
}

func (HtmlTokenUtils) value(state *HtmlParseState, rest string) string {
	if strings.HasPrefix(rest, `"`) || strings.HasPrefix(rest, "'") {
		return HtmlTokenUtils{}.quoted(state, rest, rest[:1])
	}
	return HtmlTokenUtils{}.unquoted(state, rest)
}

func (HtmlTokenUtils) quoted(state *HtmlParseState, rest string, quote string) string {
	end := strings.Index(rest[1:], quote)
	if end < 0 {
		state.Position = len(state.Source)
		return html.UnescapeString(rest[1:])
	}
	state.Position += end + len(quote) + len(quote)
	return html.UnescapeString(rest[1 : end+1])
}

func (HtmlTokenUtils) unquoted(state *HtmlParseState, rest string) string {
	end := HtmlTokenUtils{}.endOr(strings.IndexAny(rest, HtmlTokenUtils{}.spaces()+">"), len(rest))
	state.Position += end
	return html.UnescapeString(rest[:end])
}

func (HtmlTokenUtils) withAttribute(attributes []HtmlAttribute, attribute HtmlAttribute) []HtmlAttribute {
	if slices.ContainsFunc(attributes, func(existing HtmlAttribute) bool { return existing.Name == attribute.Name }) {
		return attributes
	}
	return append(attributes, attribute)
}

func (HtmlTokenUtils) TextKind() string { return "text" }

func (HtmlTokenUtils) text(state *HtmlParseState, rest string) HtmlToken {
	end := HtmlTokenUtils{}.textEnd(rest)
	state.Position += end
	return HtmlToken{Kind: HtmlTokenUtils{}.TextKind(), Text: html.UnescapeString(rest[:end])}
}

func (HtmlTokenUtils) textEnd(rest string) int {
	index := 1
	for HtmlTokenUtils{}.scanning(rest, index) {
		index++
	}
	return min(index, len(rest))
}

func (HtmlTokenUtils) scanning(rest string, index int) bool {
	return index < len(rest) && !HtmlTokenUtils{}.opensAt(rest, index)
}

func (HtmlTokenUtils) opensAt(rest string, index int) bool {
	return rest[index] == '<' && HtmlTokenUtils{}.opensMarkup(rest[index+1:])
}

func (HtmlTokenUtils) opensMarkup(rest string) bool {
	return HtmlTokenUtils{}.startsName(rest) || strings.ContainsAny(rest[:min(1, len(rest))], "/!?")
}

func (HtmlTokenUtils) consumeRaw(state *HtmlParseState, rest string, end int) string {
	if end < 0 {
		state.Position = len(state.Source)
		return rest
	}
	state.Position += end
	return rest[:end]
}
