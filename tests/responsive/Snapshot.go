//go:build responsive

package responsive

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/domsnapshot"
)

type Box struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type Element struct {
	Tag           string
	Attributes    map[string]string
	Parent        *Element
	BackendNodeID cdp.BackendNodeID
	HasLayout     bool
	Bounds        Box
	ClientBox     Box
	ScrollBox     Box
	Style         map[string]string
	Role          string
	Name          string
	DirectText    string
}

type TextBox struct {
	Owner  *Element
	Bounds Box
	Text   string
}

type Snapshot struct {
	ViewportWidth  float64
	ViewportHeight float64
	ContentWidth   float64
	ContentHeight  float64
	ContentLeft    float64
	Elements       []*Element
	TextBoxes      []TextBox
	elementsById   map[string]*Element
}

var SnapshotStyles = []string{
	"display", "visibility", "opacity", "overflow-x", "overflow-y", "position",
	"text-overflow", "transition-duration", "transition-property", "animation-duration",
	"animation-name", "animation-iteration-count", "background-color", "color-scheme",
	"padding-top", "padding-bottom", "outline-style", "outline-width",
	"border-top-style", "border-top-width", "border-bottom-style", "border-bottom-width",
	"border-left-style", "border-left-width", "border-right-style", "border-right-width",
	"direction",
}

var nonVisualTags = map[string]bool{"script": true, "style": true, "template": true, "noscript": true, "title": true}

const (
	elementNodeType = 1
	textNodeType    = 3
	noString        = -1
)

func NewSnapshot(capture domsnapshot.CaptureSnapshotResult, axNodes []*accessibility.Node) (*Snapshot, error) {
	if len(capture.Documents) == 0 {
		return nil, fmt.Errorf("the DOM snapshot has no document")
	}
	reader := snapshotReader{strings: capture.Strings, document: capture.Documents[0]}
	snapshot := &Snapshot{elementsById: map[string]*Element{}}
	byNodeIndex := reader.elements(snapshot)
	reader.addLayout(byNodeIndex)
	snapshot.TextBoxes = reader.textBoxes(byNodeIndex)
	snapshot.addAccessibility(axNodes)
	return snapshot, nil
}

type snapshotReader struct {
	strings  []string
	document *domsnapshot.DocumentSnapshot
}

func (reader snapshotReader) text(index int64) string {
	if index < 0 || int(index) >= len(reader.strings) {
		return ""
	}
	return reader.strings[index]
}

func (reader snapshotReader) elements(snapshot *Snapshot) map[int]*Element {
	nodes := reader.document.Nodes
	byNodeIndex := map[int]*Element{}
	for index := range nodes.NodeType {
		parent := byNodeIndex[int(nodes.ParentIndex[index])]
		if nodes.NodeType[index] == textNodeType {
			appendDirectText(parent, reader.text(int64(nodes.NodeValue[index])))
			continue
		}
		if nodes.NodeType[index] != elementNodeType {
			continue
		}
		element := &Element{
			Tag:           strings.ToLower(reader.text(int64(nodes.NodeName[index]))),
			Attributes:    reader.attributes(nodes.Attributes[index]),
			Parent:        parent,
			BackendNodeID: nodes.BackendNodeID[index],
			Style:         map[string]string{},
		}
		byNodeIndex[index] = element
		snapshot.Elements = append(snapshot.Elements, element)
		if id := element.Attributes["id"]; id != "" {
			snapshot.elementsById[id] = element
		}
	}
	return byNodeIndex
}

func appendDirectText(parent *Element, text string) {
	if parent == nil || strings.TrimSpace(text) == "" || nonVisualTags[parent.Tag] {
		return
	}
	parent.DirectText = strings.TrimSpace(parent.DirectText + " " + strings.Join(strings.Fields(text), " "))
}

func (reader snapshotReader) attributes(pairs domsnapshot.ArrayOfStrings) map[string]string {
	attributes := map[string]string{}
	for index := 0; index+1 < len(pairs); index += 2 {
		attributes[reader.text(pairs[index])] = reader.text(pairs[index+1])
	}
	return attributes
}

func (reader snapshotReader) addLayout(byNodeIndex map[int]*Element) {
	layout := reader.document.Layout
	for layoutIndex, nodeIndex := range layout.NodeIndex {
		element := byNodeIndex[int(nodeIndex)]
		if element == nil {
			continue
		}
		element.HasLayout = true
		element.Bounds = boxOf(layout.Bounds[layoutIndex])
		element.ClientBox = boxAt(layout.ClientRects, layoutIndex)
		element.ScrollBox = boxAt(layout.ScrollRects, layoutIndex)
		for styleIndex, value := range layout.Styles[layoutIndex] {
			element.Style[SnapshotStyles[styleIndex]] = reader.text(value)
		}
	}
}

func (reader snapshotReader) textBoxes(byNodeIndex map[int]*Element) []TextBox {
	layout := reader.document.Layout
	nodes := reader.document.Nodes
	boxes := reader.document.TextBoxes
	textBoxes := []TextBox{}
	for index, layoutIndex := range boxes.LayoutIndex {
		nodeIndex := layout.NodeIndex[layoutIndex]
		owner := byNodeIndex[int(nodes.ParentIndex[nodeIndex])]
		text := []rune(reader.text(int64(layout.Text[layoutIndex])))
		start := min(int(boxes.Start[index]), len(text))
		end := min(start+int(boxes.Length[index]), len(text))
		if owner == nil || strings.TrimSpace(string(text[start:end])) == "" {
			continue
		}
		textBoxes = append(textBoxes, TextBox{Owner: owner, Bounds: boxOf(boxes.Bounds[index]), Text: string(text[start:end])})
	}
	return textBoxes
}

func boxAt(rectangles []domsnapshot.Rectangle, index int) Box {
	if index >= len(rectangles) {
		return Box{}
	}
	return boxOf(rectangles[index])
}

func boxOf(rectangle domsnapshot.Rectangle) Box {
	if len(rectangle) < 4 {
		return Box{}
	}
	return Box{X: rectangle[0], Y: rectangle[1], Width: rectangle[2], Height: rectangle[3]}
}

func (snapshot *Snapshot) addAccessibility(axNodes []*accessibility.Node) {
	byBackendId := map[cdp.BackendNodeID]*Element{}
	for _, element := range snapshot.Elements {
		byBackendId[element.BackendNodeID] = element
	}
	for _, node := range axNodes {
		element := byBackendId[node.BackendDOMNodeID]
		if element == nil || node.Ignored {
			continue
		}
		element.Role = axString(node.Role)
		element.Name = axString(node.Name)
	}
}

func axString(value *accessibility.Value) string {
	if value == nil || len(value.Value) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal([]byte(value.Value), &text); err != nil {
		return ""
	}
	return text
}

func (snapshot *Snapshot) ElementById(id string) *Element {
	return snapshot.elementsById[id]
}

func (snapshot *Snapshot) Document() *Element {
	for _, element := range snapshot.Elements {
		if element.Tag == "html" {
			return element
		}
	}
	return nil
}

func (snapshot *Snapshot) Body() *Element {
	for _, element := range snapshot.Elements {
		if element.Tag == "body" {
			return element
		}
	}
	return nil
}

func (snapshot *Snapshot) AccessibleName(element *Element) string {
	if element.Name != "" {
		return element.Name
	}
	if label := element.Attributes["aria-label"]; label != "" {
		return label
	}
	return snapshot.labelledByText(element)
}

func (snapshot *Snapshot) labelledByText(element *Element) string {
	parts := []string{}
	for _, id := range strings.Fields(element.Attributes["aria-labelledby"]) {
		if label := snapshot.ElementById(id); label != nil {
			parts = append(parts, snapshot.subtreeText(label))
		}
	}
	return strings.Join(parts, " ")
}

func (snapshot *Snapshot) subtreeText(root *Element) string {
	parts := []string{}
	for _, element := range snapshot.Elements {
		if element.DirectText != "" && element.IsInside(root) {
			parts = append(parts, element.DirectText)
		}
	}
	return strings.Join(parts, " ")
}

func (element *Element) IsInside(ancestor *Element) bool {
	for current := element; current != nil; current = current.Parent {
		if current == ancestor {
			return true
		}
	}
	return false
}

func (element *Element) Describe() string {
	label := element.Tag
	if id := element.Attributes["id"]; id != "" {
		label += "#" + id
	}
	if element.Role != "" || element.Name != "" {
		return fmt.Sprintf("%s %q (%s)", element.Role, element.Name, label)
	}
	if element.DirectText != "" {
		return fmt.Sprintf("%s %q", label, shorten(element.DirectText))
	}
	if class := element.Attributes["class"]; class != "" {
		return label + "." + strings.Join(strings.Fields(class), ".")
	}
	return label
}

func shorten(text string) string {
	runes := []rune(text)
	if len(runes) <= 40 {
		return text
	}
	return string(runes[:40]) + "..."
}
