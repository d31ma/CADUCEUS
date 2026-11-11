//go:build responsive

package responsive

import "strings"

var interactiveTags = map[string]bool{"button": true, "select": true, "textarea": true, "summary": true}

var interactiveRoles = map[string]bool{
	"button": true, "link": true, "menuitem": true, "menuitemradio": true, "menuitemcheckbox": true,
	"tab": true, "switch": true, "checkbox": true, "radio": true, "option": true, "combobox": true,
	"textbox": true, "searchbox": true, "slider": true, "spinbutton": true,
}

var clippingOverflows = map[string]bool{"hidden": true, "clip": true, "auto": true, "scroll": true}

func (snapshot *Snapshot) IsVisible(element *Element) bool {
	if !element.HasLayout || element.Style["visibility"] != "visible" {
		return false
	}
	if isCollapsed(element.Bounds) || !snapshot.intersectsViewport(element.Bounds) {
		return false
	}
	return !hiddenByAncestor(element)
}

func isCollapsed(box Box) bool {
	return box.Width <= 1 || box.Height <= 1
}

func (snapshot *Snapshot) intersectsViewport(box Box) bool {
	return box.X < snapshot.ViewportWidth && box.X+box.Width > 0 && box.Y+box.Height > 0
}

func hiddenByAncestor(element *Element) bool {
	for current := element; current != nil; current = current.Parent {
		if current.Style["opacity"] == "0" {
			return true
		}
		if current != element && clipsContent(current) && clipsAway(current.Bounds, element.Bounds) {
			return true
		}
	}
	return false
}

func clipsContent(element *Element) bool {
	return clippingOverflows[element.Style["overflow-x"]] || clippingOverflows[element.Style["overflow-y"]]
}

func clipsAway(container Box, inner Box) bool {
	return isCollapsed(container) || OverlapArea(container, inner) <= 0
}

func (element *Element) IsInert() bool {
	for current := element; current != nil; current = current.Parent {
		if _, inert := current.Attributes["inert"]; inert {
			return true
		}
	}
	return false
}

func (element *Element) IsInteractive() bool {
	if element.Tag == "a" {
		_, hasHref := element.Attributes["href"]
		return hasHref
	}
	if element.Tag == "input" {
		return element.Attributes["type"] != "hidden"
	}
	if interactiveTags[element.Tag] || interactiveRoles[element.Attributes["role"]] {
		return true
	}
	if tabIndex, has := element.Attributes["tabindex"]; has && strings.TrimSpace(tabIndex) != "-1" {
		return true
	}
	editable, hasEditable := element.Attributes["contenteditable"]
	return hasEditable && editable != "false"
}

func (snapshot *Snapshot) TargetBox(element *Element) Box {
	if !element.IsFormField() {
		return element.Bounds
	}
	target := element.Bounds
	for current := element.Parent; current != nil && !snapshot.holdsForeignText(current, element); current = current.Parent {
		if current.HasLayout {
			target = current.Bounds
		}
		if current.Tag == "label" || strings.HasPrefix(current.Tag, "w-") {
			break
		}
	}
	return target
}

func (snapshot *Snapshot) holdsForeignText(container *Element, field *Element) bool {
	for _, element := range snapshot.Elements {
		if element.DirectText != "" && element.IsInside(container) && !snapshot.labelsField(element, field) {
			return true
		}
	}
	return false
}

func (snapshot *Snapshot) labelsField(text *Element, field *Element) bool {
	for current := text; current != nil; current = current.Parent {
		if current.Tag == "label" && (field.IsInside(current) || current.Attributes["for"] == field.Attributes["id"]) {
			return true
		}
	}
	return text.IsInside(field)
}

func (snapshot *Snapshot) IsTarget(element *Element) bool {
	if !element.IsInteractive() || element.IsInert() || !element.HasLayout || element.Style["visibility"] != "visible" {
		return false
	}
	box := snapshot.TargetBox(element)
	if isCollapsed(box) || !snapshot.intersectsViewport(box) {
		return false
	}
	if element.IsFormField() && element.Parent != nil {
		return !hiddenByAncestor(element.Parent)
	}
	return !hiddenByAncestor(element)
}

func (element *Element) IsFormField() bool {
	return element.Tag == "input" || element.Tag == "textarea" || element.Tag == "select"
}

func OverlapArea(first Box, second Box) float64 {
	width := min(first.X+first.Width, second.X+second.Width) - max(first.X, second.X)
	height := min(first.Y+first.Height, second.Y+second.Height) - max(first.Y, second.Y)
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}
