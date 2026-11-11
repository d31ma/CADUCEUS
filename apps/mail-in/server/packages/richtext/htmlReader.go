package richtext

import (
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

type htmlReader struct {
	builder      *blockBuilder
	inlineImages map[string]string
	rules        map[string]elementRule
	nodeEnters   map[html.NodeType]func(*html.Node) bool
	headingLevel int
	preDepth     int
	quoteDepth   int
	itemDepth    int
	bold         int
	italic       int
	code         int
	strike       int
	links        []string
	orderedLists []bool
	row          *html.Node
}

func newHtmlReader(builder *blockBuilder, inlineImages map[string]string) *htmlReader {
	reader := &htmlReader{builder: builder, inlineImages: inlineImages, links: []string{""}, orderedLists: []bool{false}}
	reader.rules = reader.buildRules()
	reader.nodeEnters = map[html.NodeType]func(*html.Node) bool{
		html.DocumentNode: reader.descend,
		html.ElementNode:  reader.enterElement,
		html.TextNode:     reader.text,
	}
	return reader
}

func (reader *htmlReader) read(document *html.Node) {
	treeWalker{enter: reader.enter, leave: reader.leave}.walk(document)
}

func (reader *htmlReader) enter(node *html.Node) bool {
	return reader.enterFor(node.Type)(node)
}

func (reader *htmlReader) enterFor(nodeType html.NodeType) func(*html.Node) bool {
	handler, known := reader.nodeEnters[nodeType]
	if !known {
		return reader.skip
	}
	return handler
}

func (reader *htmlReader) leave(node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}
	reader.ruleFor(node).leave(node)
}

func (reader *htmlReader) enterElement(node *html.Node) bool {
	return reader.ruleFor(node).enter(node)
}

func (reader *htmlReader) ruleFor(node *html.Node) elementRule {
	rule, known := reader.rules[node.Data]
	if !known {
		return elementRule{enter: reader.descend, leave: reader.ignore}
	}
	return rule
}

func (reader *htmlReader) descend(*html.Node) bool {
	return true
}

func (reader *htmlReader) skip(*html.Node) bool {
	return false
}

func (reader *htmlReader) ignore(*html.Node) {}

func (reader *htmlReader) buildRules() map[string]elementRule {
	tags := htmlTags{}
	rules := map[string]elementRule{
		tags.lineBreak(): {enter: reader.lineBreak, leave: reader.ignore},
		"hr":             {enter: reader.divider, leave: reader.ignore, block: true},
		"img":            {enter: reader.image, leave: reader.ignore, block: true},
		"a":              {enter: reader.enterLink, leave: reader.leaveLink},
		"tr":             {enter: reader.enterRow, leave: reader.leaveRow, block: true},
		"ul":             reader.listRule(false),
		"ol":             reader.listRule(true),
		"li":             reader.blockRule(&reader.itemDepth),
		"blockquote":     reader.blockRule(&reader.quoteDepth),
		"pre":            reader.blockRule(&reader.preDepth),
	}
	reader.addRules(rules, tags.cells(), elementRule{enter: reader.enterCell, leave: reader.leaveCell, block: true})
	reader.addRules(rules, tags.boundaries(), reader.blockRule(new(int)))
	reader.addRules(rules, tags.bold(), reader.styleRule(&reader.bold))
	reader.addRules(rules, tags.italic(), reader.styleRule(&reader.italic))
	reader.addRules(rules, tags.strike(), reader.styleRule(&reader.strike))
	reader.addRules(rules, tags.code(), reader.styleRule(&reader.code))
	reader.addRules(rules, tags.dropped(), elementRule{enter: reader.skip, leave: reader.ignore})
	reader.addHeadings(rules, tags.headings())
	return rules
}

func (reader *htmlReader) addRules(rules map[string]elementRule, tags []string, rule elementRule) {
	for _, tag := range tags {
		rules[tag] = rule
	}
}

func (reader *htmlReader) addHeadings(rules map[string]elementRule, tags []string) {
	for index, tag := range tags {
		rules[tag] = reader.headingRule(index + 1)
	}
}

func (reader *htmlReader) styleRule(count *int) elementRule {
	return elementRule{
		enter: func(*html.Node) bool {
			*count++
			return true
		},
		leave: func(*html.Node) {
			*count--
		},
	}
}

func (reader *htmlReader) blockRule(count *int) elementRule {
	return elementRule{
		enter: func(*html.Node) bool {
			*count++
			reader.reshape()
			return true
		},
		leave: func(*html.Node) {
			*count--
			reader.reshape()
		},
		block: true,
	}
}

func (reader *htmlReader) headingRule(level int) elementRule {
	return elementRule{
		enter: func(*html.Node) bool {
			reader.headingLevel = level
			reader.reshape()
			return true
		},
		leave: func(*html.Node) {
			reader.headingLevel = 0
			reader.reshape()
		},
		block: true,
	}
}

func (reader *htmlReader) listRule(ordered bool) elementRule {
	return elementRule{
		enter: func(*html.Node) bool {
			reader.orderedLists = append(reader.orderedLists, ordered)
			reader.reshape()
			return true
		},
		leave: func(*html.Node) {
			reader.orderedLists = reader.orderedLists[:len(reader.orderedLists)-1]
			reader.reshape()
		},
		block: true,
	}
}

func (reader *htmlReader) lineBreak(*html.Node) bool {
	reader.builder.newline()
	return false
}

func (reader *htmlReader) divider(*html.Node) bool {
	reader.builder.addDivider()
	return false
}

func (reader *htmlReader) image(node *html.Node) bool {
	source := strings.TrimSpace(reader.attribute(node, "src"))
	reader.builder.addImage(ImageRef{
		Alt:          reader.attribute(node, "alt"),
		AttachmentId: reader.attachmentFor(source),
		RemoteUrl:    schemes{}.image(source),
	})
	return false
}

func (reader *htmlReader) attachmentFor(source string) string {
	scheme := "cid"
	if (schemes{}).schemeOf(source) != scheme {
		return ""
	}
	return reader.inlineImages[reader.unescaped(source[len(scheme)+1:])]
}

func (reader *htmlReader) unescaped(text string) string {
	value, err := url.PathUnescape(text)
	if err != nil {
		return text
	}
	return value
}

func (reader *htmlReader) attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func (reader *htmlReader) enterLink(node *html.Node) bool {
	reader.links = append(reader.links, schemes{}.link(reader.attribute(node, "href")))
	return true
}

func (reader *htmlReader) leaveLink(*html.Node) {
	reader.links = reader.links[:len(reader.links)-1]
}

func (reader *htmlReader) enterRow(node *html.Node) bool {
	reader.startRowWhen(reader.row == nil && reader.isDataRow(node), node)
	return true
}

func (reader *htmlReader) startRowWhen(starts bool, node *html.Node) {
	if !starts {
		reader.reshape()
		return
	}
	reader.row = node
	reader.builder.startRow()
}

func (reader *htmlReader) leaveRow(node *html.Node) {
	reader.endRowWhen(node == reader.row)
	reader.reshape()
}

func (reader *htmlReader) endRowWhen(ends bool) {
	if !ends {
		return
	}
	reader.row = nil
	reader.builder.endRow()
}

func (reader *htmlReader) enterCell(node *html.Node) bool {
	reader.startCellWhen(node.Parent == reader.row)
	return true
}

func (reader *htmlReader) startCellWhen(starts bool) {
	if !starts {
		reader.reshape()
		return
	}
	reader.builder.startCell()
}

func (reader *htmlReader) leaveCell(*html.Node) {
	reader.reshape()
}

func (htmlReader) minimumCells() int {
	return 2
}

func (reader *htmlReader) isDataRow(row *html.Node) bool {
	cells := reader.cellsOf(row)
	return len(cells) >= reader.minimumCells() && !slices.ContainsFunc(cells, reader.hasBlockChild)
}

func (reader *htmlReader) cellsOf(row *html.Node) []*html.Node {
	cells := []*html.Node{}
	for child := range row.ChildNodes() {
		cells = reader.appendCell(cells, child)
	}
	return cells
}

func (reader *htmlReader) appendCell(cells []*html.Node, node *html.Node) []*html.Node {
	if node.Type != html.ElementNode || !slices.Contains(htmlTags{}.cells(), node.Data) {
		return cells
	}
	return append(cells, node)
}

func (reader *htmlReader) hasBlockChild(cell *html.Node) bool {
	for child := range cell.ChildNodes() {
		if child.Type == html.ElementNode && reader.ruleFor(child).block {
			return true
		}
	}
	return false
}

func (reader *htmlReader) text(node *html.Node) bool {
	reader.writeText(node.Data)
	return false
}

func (reader *htmlReader) writeText(data string) {
	if reader.preDepth > 0 {
		reader.builder.addLines(strings.Split(data, reader.builder.lineBreak()), reader.style())
		return
	}
	reader.builder.addFlowing(data, reader.style())
}

func (reader *htmlReader) style() Span {
	return Span{
		Bold:   reader.bold > 0,
		Italic: reader.italic > 0,
		Code:   reader.code > 0,
		Strike: reader.strike > 0,
		Href:   reader.links[len(reader.links)-1],
	}
}

func (reader *htmlReader) reshape() {
	reader.builder.setShape(reader.shape())
}

func (reader *htmlReader) shape() Block {
	kinds := Kinds{}
	candidates := []shapeCandidate{
		{active: reader.headingLevel > 0, shape: Block{Kind: kinds.Heading(), Level: reader.headingLevel}},
		{active: reader.preDepth > 0, shape: Block{Kind: kinds.Code()}},
		{active: reader.itemDepth > 0, shape: Block{Kind: kinds.ListItem(), Level: reader.level(len(reader.orderedLists) - 1), Ordered: reader.orderedLists[len(reader.orderedLists)-1]}},
		{active: reader.quoteDepth > 0, shape: Block{Kind: kinds.Quote(), Level: reader.level(reader.quoteDepth)}},
		{active: true, shape: Block{Kind: kinds.Paragraph()}},
	}
	return candidates[slices.IndexFunc(candidates, shapeCandidate.isActive)].shape
}

func (reader *htmlReader) level(depth int) int {
	return min(max(depth, 1), Limits{}.MaxDepth())
}
