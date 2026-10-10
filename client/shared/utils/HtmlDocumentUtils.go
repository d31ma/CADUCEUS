import (
	"slices"
	"strings"
)

// HtmlDocumentUtils builds the body of an HTML document from untrusted mail
// markup, edits it and writes it back, in the places where the reader used
// DOMParser before. Content before the body that belongs in the head (styles,
// titles, metadata) is dropped, as document.body.innerHTML dropped it.
type HtmlDocumentUtils struct{}

func (HtmlDocumentUtils) bodyName() string { return "body" }

func (HtmlDocumentUtils) Parse(source string) *HtmlNode {
	body := HtmlDocumentUtils{}.element(HtmlDocumentUtils{}.bodyName())
	state := &HtmlParseState{Source: source, Body: body, Open: []*HtmlNode{body}}
	HtmlDocumentUtils{}.build(state)
	return body
}

func (HtmlDocumentUtils) Serialize(root *HtmlNode) string {
	var builder strings.Builder
	HtmlDocumentUtils{}.writeChildren(&builder, root)
	return builder.String()
}

func (HtmlDocumentUtils) Elements(root *HtmlNode) []*HtmlNode {
	found := []*HtmlNode{}
	HtmlDocumentUtils{}.collect(root, &found)
	return found
}

func (HtmlDocumentUtils) Attribute(node *HtmlNode, name string) (string, bool) {
	return HtmlDocumentUtils{}.attributeAt(node, HtmlDocumentUtils{}.attributeIndex(node, name))
}

func (HtmlDocumentUtils) SetAttribute(node *HtmlNode, name string, value string) {
	HtmlDocumentUtils{}.setAt(node, HtmlDocumentUtils{}.attributeIndex(node, name), HtmlAttribute{Name: name, Value: value})
}

func (HtmlDocumentUtils) RemoveAttribute(node *HtmlNode, name string) {
	node.Attributes = slices.DeleteFunc(node.Attributes, func(attribute HtmlAttribute) bool { return attribute.Name == name })
}

func (HtmlDocumentUtils) Remove(node *HtmlNode) {
	HtmlDocumentUtils{}.detach(node, node.Parent)
}

func (HtmlDocumentUtils) TextContent(node *HtmlNode) string {
	var builder strings.Builder
	HtmlDocumentUtils{}.writeText(&builder, node)
	return builder.String()
}

func (HtmlDocumentUtils) ReplaceText(node *HtmlNode, text string) {
	node.Children = []*HtmlNode{{Kind: HtmlTokenUtils{}.TextKind(), Text: text, Parent: node}}
}

func (HtmlDocumentUtils) names(list string) []string { return strings.Fields(list) }

func (HtmlDocumentUtils) voidElements() []string {
	return HtmlDocumentUtils{}.names("area base basefont bgsound br col embed frame hr img input keygen link meta param source track wbr")
}

func (HtmlDocumentUtils) rawTextElements() []string {
	return HtmlDocumentUtils{}.names("style script xmp iframe noembed noframes plaintext textarea title")
}

func (HtmlDocumentUtils) rawSerializedElements() []string {
	return HtmlDocumentUtils{}.names("style script xmp iframe noembed noframes plaintext")
}

func (HtmlDocumentUtils) headElements() []string {
	return HtmlDocumentUtils{}.names("base basefont bgsound link meta title style script noscript template noframes html head")
}

func (HtmlDocumentUtils) documentElements() []string {
	return HtmlDocumentUtils{}.names("html head body")
}

func (HtmlDocumentUtils) paragraphClosers() []string {
	return HtmlDocumentUtils{}.names("address article aside blockquote center details dialog dir div dl fieldset figcaption figure footer form h1 h2 h3 h4 h5 h6 header hgroup hr listing main menu nav ol p plaintext pre section summary table ul xmp li dd dt")
}

func (HtmlDocumentUtils) headings() []string {
	return HtmlDocumentUtils{}.names("h1 h2 h3 h4 h5 h6")
}

func (HtmlDocumentUtils) foreignElements() []string {
	return HtmlDocumentUtils{}.names("svg math")
}

func (HtmlDocumentUtils) generalScope() []string {
	return HtmlDocumentUtils{}.names("applet caption table td th marquee object template svg math")
}

func (HtmlDocumentUtils) separator() rune { return ',' }

func (HtmlDocumentUtils) rules(table string) map[string][]string {
	rules := map[string][]string{}
	for _, rule := range strings.Fields(table) {
		name, values, _ := strings.Cut(rule, "=")
		rules[name] = strings.Split(values, string(HtmlDocumentUtils{}.separator()))
	}
	return rules
}

func (HtmlDocumentUtils) implicitClosers() map[string][]string {
	return HtmlDocumentUtils{}.rules("li=li dd=dd,dt dt=dd,dt option=option optgroup=option,optgroup tr=tr,td,th td=td,th th=td,th tbody=tbody,thead,tfoot,tr,td,th thead=tbody,thead,tfoot,tr,td,th tfoot=tbody,thead,tfoot,tr,td,th a=a")
}

func (HtmlDocumentUtils) scopes() map[string][]string {
	general := strings.Join(HtmlDocumentUtils{}.generalScope(), string(HtmlDocumentUtils{}.separator()))
	tables := "table,template"
	return HtmlDocumentUtils{}.rules("li=" + general + ",ul,ol dd=" + general + ",ul,ol,dl dt=" + general + ",ul,ol,dl p=" + general + ",button tr=" + tables + " td=" + tables + " th=" + tables + " tbody=" + tables + " thead=" + tables + " tfoot=" + tables + " option=select optgroup=select")
}

func (HtmlDocumentUtils) scopeFor(name string) []string {
	found, known := HtmlDocumentUtils{}.scopes()[name]
	return HtmlDocumentUtils{}.knownScope(found, known)
}

func (HtmlDocumentUtils) knownScope(found []string, known bool) []string {
	if known {
		return found
	}
	return HtmlDocumentUtils{}.generalScope()
}

func (HtmlDocumentUtils) elementKind() string { return "element" }

func (HtmlDocumentUtils) element(name string) *HtmlNode {
	return &HtmlNode{Kind: HtmlDocumentUtils{}.elementKind(), Name: name, Attributes: []HtmlAttribute{}}
}

func (HtmlDocumentUtils) build(state *HtmlParseState) {
	handlers := map[bool]func(*HtmlParseState, HtmlToken){false: HtmlDocumentUtils{}.headToken, true: HtmlDocumentUtils{}.bodyToken}
	for state.Position < len(state.Source) {
		token := HtmlTokenUtils{}.Next(state)
		state.InBody = HtmlDocumentUtils{}.inBody(state.InBody, token)
		handlers[state.InBody](state, token)
	}
}

func (HtmlDocumentUtils) inBody(inBody bool, token HtmlToken) bool {
	return inBody || HtmlDocumentUtils{}.startsBody(token)
}

func (HtmlDocumentUtils) startsBody(token HtmlToken) bool {
	switch token.Kind {
	case HtmlTokenUtils{}.TextKind():
		return strings.TrimSpace(token.Text) != ""
	case HtmlTokenUtils{}.StartKind():
		return !slices.Contains(HtmlDocumentUtils{}.headElements(), token.Name)
	}
	return false
}

func (HtmlDocumentUtils) headToken(state *HtmlParseState, token HtmlToken) {
	start := token.Kind == HtmlTokenUtils{}.StartKind()
	skips := HtmlDocumentUtils{}.skipsRaw(token.Name)
	if start && skips {
		HtmlTokenUtils{}.RawText(state, token.Name)
		HtmlTokenUtils{}.Next(state)
	}
}

func (HtmlDocumentUtils) noScript() string { return "noscript" }

func (HtmlDocumentUtils) template() string { return "template" }

func (HtmlDocumentUtils) skipsRaw(name string) bool {
	utils := HtmlDocumentUtils{}
	if slices.Contains(utils.rawTextElements(), name) || name == utils.noScript() || name == utils.template() {
		return true
	}
	return false
}

func (HtmlDocumentUtils) bodyToken(state *HtmlParseState, token HtmlToken) {
	switch token.Kind {
	case HtmlTokenUtils{}.TextKind():
		HtmlDocumentUtils{}.appendText(HtmlDocumentUtils{}.current(state), token.Text)
	case HtmlTokenUtils{}.CommentKind():
		HtmlDocumentUtils{}.appendChild(HtmlDocumentUtils{}.current(state), &HtmlNode{Kind: HtmlTokenUtils{}.CommentKind(), Text: token.Text})
	case HtmlTokenUtils{}.StartKind():
		HtmlDocumentUtils{}.startTag(state, token)
	case HtmlTokenUtils{}.EndKind():
		HtmlDocumentUtils{}.endTag(state, token.Name)
	}
}

func (HtmlDocumentUtils) current(state *HtmlParseState) *HtmlNode {
	return state.Open[len(state.Open)-1]
}

func (HtmlDocumentUtils) appendText(parent *HtmlNode, text string) {
	last := len(parent.Children) - 1
	if last >= 0 && parent.Children[last].Kind == HtmlTokenUtils{}.TextKind() {
		parent.Children[last].Text += text
		return
	}
	HtmlDocumentUtils{}.appendChild(parent, &HtmlNode{Kind: HtmlTokenUtils{}.TextKind(), Text: text})
}

func (HtmlDocumentUtils) appendChild(parent *HtmlNode, child *HtmlNode) {
	child.Parent = parent
	parent.Children = append(parent.Children, child)
}

func (HtmlDocumentUtils) startTag(state *HtmlParseState, token HtmlToken) {
	if slices.Contains(HtmlDocumentUtils{}.documentElements(), token.Name) {
		return
	}
	HtmlDocumentUtils{}.closeImplied(state, token.Name)
	element := &HtmlNode{Kind: HtmlDocumentUtils{}.elementKind(), Name: token.Name, Attributes: token.Attributes}
	HtmlDocumentUtils{}.appendChild(HtmlDocumentUtils{}.current(state), element)
	HtmlDocumentUtils{}.content(state, element, HtmlDocumentUtils{}.foreignSelfClosing(state, token.SelfClosing))
}

func (HtmlDocumentUtils) content(state *HtmlParseState, element *HtmlNode, selfClosed bool) {
	switch {
	case selfClosed:
	case slices.Contains(HtmlDocumentUtils{}.voidElements(), element.Name):
	case slices.Contains(HtmlDocumentUtils{}.rawTextElements(), element.Name):
		HtmlDocumentUtils{}.rawContent(state, element)
	default:
		state.Open = append(state.Open, element)
	}
}

func (HtmlDocumentUtils) foreignSelfClosing(state *HtmlParseState, selfClosing bool) bool {
	return selfClosing && slices.ContainsFunc(state.Open, func(node *HtmlNode) bool { return slices.Contains(HtmlDocumentUtils{}.foreignElements(), node.Name) })
}

func (HtmlDocumentUtils) rawContent(state *HtmlParseState, element *HtmlNode) {
	text := HtmlTokenUtils{}.RawText(state, element.Name)
	HtmlDocumentUtils{}.appendRaw(element, text)
	HtmlTokenUtils{}.Next(state)
}

func (HtmlDocumentUtils) appendRaw(element *HtmlNode, text string) {
	if text != "" {
		HtmlDocumentUtils{}.appendChild(element, &HtmlNode{Kind: HtmlTokenUtils{}.TextKind(), Text: text})
	}
}

func (HtmlDocumentUtils) closeImplied(state *HtmlParseState, name string) {
	HtmlDocumentUtils{}.closeParagraph(state, name)
	HtmlDocumentUtils{}.closeHeading(state, name)
	HtmlDocumentUtils{}.popThrough(state, HtmlDocumentUtils{}.implicitClosers()[name], HtmlDocumentUtils{}.scopeFor(name))
}

func (HtmlDocumentUtils) paragraph() string { return "p" }

func (HtmlDocumentUtils) closeParagraph(state *HtmlParseState, name string) {
	if slices.Contains(HtmlDocumentUtils{}.paragraphClosers(), name) {
		HtmlDocumentUtils{}.popThrough(state, []string{HtmlDocumentUtils{}.paragraph()}, HtmlDocumentUtils{}.scopeFor(HtmlDocumentUtils{}.paragraph()))
	}
}

func (HtmlDocumentUtils) closeHeading(state *HtmlParseState, name string) {
	utils := HtmlDocumentUtils{}
	if slices.Contains(utils.headings(), name) && slices.Contains(utils.headings(), utils.current(state).Name) {
		state.Open = state.Open[:len(state.Open)-1]
	}
}

func (HtmlDocumentUtils) popThrough(state *HtmlParseState, targets []string, boundaries []string) {
	index := HtmlDocumentUtils{}.openIndex(state.Open, targets, boundaries)
	state.Open = state.Open[:max(index, 1)]
}

func (HtmlDocumentUtils) openIndex(open []*HtmlNode, targets []string, boundaries []string) int {
	index := len(open) - 1
	for HtmlDocumentUtils{}.searching(open, index, targets, boundaries) {
		index--
	}
	return HtmlDocumentUtils{}.matchedIndex(open, targets, index)
}

func (HtmlDocumentUtils) searching(open []*HtmlNode, index int, targets []string, boundaries []string) bool {
	return index > 0 && !HtmlDocumentUtils{}.stopsSearch(open[index], targets, boundaries)
}

func (HtmlDocumentUtils) stopsSearch(node *HtmlNode, targets []string, boundaries []string) bool {
	return slices.Contains(targets, node.Name) || slices.Contains(boundaries, node.Name)
}

func (HtmlDocumentUtils) matchedIndex(open []*HtmlNode, targets []string, index int) int {
	if index > 0 && slices.Contains(targets, open[index].Name) {
		return index
	}
	return len(open)
}

func (HtmlDocumentUtils) lineBreak() string { return "br" }

func (HtmlDocumentUtils) endTag(state *HtmlParseState, name string) {
	switch {
	case slices.Contains(HtmlDocumentUtils{}.documentElements(), name):
	case name == HtmlDocumentUtils{}.lineBreak():
		HtmlDocumentUtils{}.appendChild(HtmlDocumentUtils{}.current(state), HtmlDocumentUtils{}.element(name))
	case name == HtmlDocumentUtils{}.paragraph():
		HtmlDocumentUtils{}.endParagraph(state, HtmlDocumentUtils{}.openIndex(state.Open, []string{name}, HtmlDocumentUtils{}.scopeFor(name)))
	default:
		HtmlDocumentUtils{}.popThrough(state, []string{name}, HtmlDocumentUtils{}.generalScope())
	}
}

func (HtmlDocumentUtils) endParagraph(state *HtmlParseState, index int) {
	if index == len(state.Open) {
		HtmlDocumentUtils{}.appendChild(HtmlDocumentUtils{}.current(state), HtmlDocumentUtils{}.element(HtmlDocumentUtils{}.paragraph()))
		return
	}
	state.Open = state.Open[:index]
}

func (HtmlDocumentUtils) collect(node *HtmlNode, found *[]*HtmlNode) {
	for _, child := range node.Children {
		HtmlDocumentUtils{}.collectElement(child, found)
	}
}

func (HtmlDocumentUtils) collectElement(node *HtmlNode, found *[]*HtmlNode) {
	if node.Kind == HtmlDocumentUtils{}.elementKind() {
		*found = append(*found, node)
		HtmlDocumentUtils{}.collect(node, found)
	}
}

func (HtmlDocumentUtils) attributeIndex(node *HtmlNode, name string) int {
	return slices.IndexFunc(node.Attributes, func(attribute HtmlAttribute) bool { return attribute.Name == name })
}

func (HtmlDocumentUtils) attributeAt(node *HtmlNode, index int) (string, bool) {
	if index < 0 {
		return "", false
	}
	return node.Attributes[index].Value, true
}

func (HtmlDocumentUtils) setAt(node *HtmlNode, index int, attribute HtmlAttribute) {
	if index < 0 {
		node.Attributes = append(node.Attributes, attribute)
		return
	}
	node.Attributes[index] = attribute
}

func (HtmlDocumentUtils) detach(node *HtmlNode, parent *HtmlNode) {
	if parent != nil {
		parent.Children = slices.DeleteFunc(parent.Children, func(child *HtmlNode) bool { return child == node })
		node.Parent = nil
	}
}

func (HtmlDocumentUtils) writeText(builder *strings.Builder, node *HtmlNode) {
	builder.WriteString(HtmlDocumentUtils{}.ownText(node))
	for _, child := range node.Children {
		HtmlDocumentUtils{}.writeText(builder, child)
	}
}

func (HtmlDocumentUtils) ownText(node *HtmlNode) string {
	if node.Kind == HtmlTokenUtils{}.TextKind() {
		return node.Text
	}
	return ""
}

func (HtmlDocumentUtils) writeChildren(builder *strings.Builder, node *HtmlNode) {
	for _, child := range node.Children {
		HtmlDocumentUtils{}.writeNode(builder, child, node)
	}
}

func (HtmlDocumentUtils) writeNode(builder *strings.Builder, node *HtmlNode, parent *HtmlNode) {
	switch node.Kind {
	case HtmlTokenUtils{}.TextKind():
		builder.WriteString(HtmlDocumentUtils{}.textFor(node.Text, parent))
	case HtmlTokenUtils{}.CommentKind():
		builder.WriteString("<!--" + node.Text + "-->")
	default:
		HtmlDocumentUtils{}.writeElement(builder, node)
	}
}

func (HtmlDocumentUtils) escapes(table string) *strings.Replacer {
	return strings.NewReplacer(strings.Split(table, "|")...)
}

func (HtmlDocumentUtils) textFor(text string, parent *HtmlNode) string {
	if slices.Contains(HtmlDocumentUtils{}.rawSerializedElements(), parent.Name) {
		return text
	}
	return HtmlDocumentUtils{}.escapes("&|&amp;| |&nbsp;|<|&lt;|>|&gt;").Replace(text)
}

func (HtmlDocumentUtils) tagEnd() string { return ">" }

func (HtmlDocumentUtils) writeElement(builder *strings.Builder, node *HtmlNode) {
	escapes := HtmlDocumentUtils{}.escapes(`&|&amp;|` + " " + `|&nbsp;|"|&quot;|<|&lt;|>|&gt;`)
	builder.WriteString("<" + node.Name)
	for _, attribute := range node.Attributes {
		builder.WriteString(" " + attribute.Name + `="` + escapes.Replace(attribute.Value) + `"`)
	}
	builder.WriteString(HtmlDocumentUtils{}.tagEnd())
	HtmlDocumentUtils{}.writeEnd(builder, node)
}

func (HtmlDocumentUtils) writeEnd(builder *strings.Builder, node *HtmlNode) {
	if !slices.Contains(HtmlDocumentUtils{}.voidElements(), node.Name) {
		HtmlDocumentUtils{}.writeChildren(builder, node)
		builder.WriteString("</" + node.Name + HtmlDocumentUtils{}.tagEnd())
	}
}
