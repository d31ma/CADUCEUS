//go:build responsive

package responsive

import (
	"fmt"
	"strings"
)

type ShellLandmarks struct {
	Main    *Element
	Rail    *Element
	Sidebar *Element
	Thread  *Element
}

const (
	narrowLayoutLimit    = 768
	threadColumnWidth    = 1200
	touchTargetSize      = 44
	pointerTargetSize    = 24
	motionTolerance      = 0.01
	darkLuminanceLimit   = 0.2
	overlapTolerance     = 1.0
	textOverlapShare     = 0.2
	maximumOverflowNames = 5
)

func CheckHorizontalScroll(snapshot *Snapshot) []string {
	if snapshot.ContentWidth <= snapshot.ViewportWidth+0.5 && snapshot.ContentLeft >= -0.5 {
		return nil
	}
	names := []string{}
	for _, element := range pageOverflowLeaves(snapshot) {
		names = append(names, fmt.Sprintf("%s spans x=%.0f..%.0f", element.Describe(), element.Bounds.X, element.Bounds.X+element.Bounds.Width))
	}
	return []string{fmt.Sprintf("page scrolls sideways: content is %.0f px wide (starts at x=%.0f) in a %.0f px viewport; elements past the edge: %s",
		snapshot.ContentWidth, snapshot.ContentLeft, snapshot.ViewportWidth, strings.Join(names, "; "))}
}

func pageOverflowLeaves(snapshot *Snapshot) []*Element {
	overflowing := map[*Element]bool{}
	for _, element := range snapshot.Elements {
		if snapshot.overflowsPage(element) {
			overflowing[element] = true
		}
	}
	leaves := []*Element{}
	for _, element := range snapshot.Elements {
		if overflowing[element] && !hasOverflowingChild(element, overflowing) && len(leaves) < maximumOverflowNames {
			leaves = append(leaves, element)
		}
	}
	return leaves
}

func (snapshot *Snapshot) overflowsPage(element *Element) bool {
	if !element.HasLayout || element.Style["visibility"] != "visible" || isCollapsed(element.Bounds) {
		return false
	}
	right := element.Bounds.X + element.Bounds.Width
	if right <= snapshot.ViewportWidth+1 && element.Bounds.X >= -1 {
		return false
	}
	return !insideClippingAncestor(element)
}

func insideClippingAncestor(element *Element) bool {
	for current := element.Parent; current != nil; current = current.Parent {
		if current.Tag != "html" && current.Tag != "body" && clipsContent(current) {
			return true
		}
	}
	return false
}

func hasOverflowingChild(parent *Element, overflowing map[*Element]bool) bool {
	for element := range overflowing {
		if element.Parent == parent {
			return true
		}
	}
	return false
}

func CheckTargetSizes(snapshot *Snapshot, minimum float64) []string {
	findings := []string{}
	for _, element := range snapshot.Elements {
		if !snapshot.IsTarget(element) {
			continue
		}
		box := snapshot.TargetBox(element)
		if box.Width < minimum-0.5 || box.Height < minimum-0.5 {
			findings = append(findings, fmt.Sprintf("touch target %s is %.0fx%.0f px, needs at least %.0fx%.0f",
				snapshot.describeWithName(element), box.Width, box.Height, minimum, minimum))
		}
	}
	return findings
}

func (snapshot *Snapshot) describeWithName(element *Element) string {
	if element.Name == "" {
		element.Name = snapshot.AccessibleName(element)
	}
	return element.Describe()
}

func CheckClippedText(snapshot *Snapshot) []string {
	findings := []string{}
	for _, element := range snapshot.Elements {
		if element.DirectText == "" || element.IsFormField() || element.ClientBox.Width <= 0 {
			continue
		}
		if strings.Contains(element.Style["text-overflow"], "ellipsis") || !snapshot.IsVisible(element) {
			continue
		}
		if element.ScrollBox.Width > element.ClientBox.Width+1 {
			findings = append(findings, fmt.Sprintf("text in %s is clipped or spills out: content %.0f px wide in a %.0f px box (overflow-x: %s)",
				element.Describe(), element.ScrollBox.Width, element.ClientBox.Width, element.Style["overflow-x"]))
		}
	}
	return findings
}

func CheckOverlappingText(snapshot *Snapshot) []string {
	visible := []TextBox{}
	for _, textBox := range snapshot.TextBoxes {
		if snapshot.IsVisible(textBox.Owner) && !isCollapsed(textBox.Bounds) && snapshot.intersectsViewport(textBox.Bounds) {
			visible = append(visible, textBox)
		}
	}
	findings := []string{}
	reported := map[[2]*Element]bool{}
	for first := range visible {
		for second := first + 1; second < len(visible); second++ {
			finding := textOverlap(visible[first], visible[second], reported)
			if finding != "" {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

func textOverlap(first TextBox, second TextBox, reported map[[2]*Element]bool) string {
	if first.Owner.IsInside(second.Owner) || second.Owner.IsInside(first.Owner) || reported[[2]*Element{first.Owner, second.Owner}] {
		return ""
	}
	area := OverlapArea(first.Bounds, second.Bounds)
	smaller := min(first.Bounds.Width*first.Bounds.Height, second.Bounds.Width*second.Bounds.Height)
	if area <= 4 || area < textOverlapShare*smaller {
		return ""
	}
	reported[[2]*Element{first.Owner, second.Owner}] = true
	return fmt.Sprintf("text %q in %s overlaps text %q in %s", shorten(first.Text), first.Owner.Describe(), shorten(second.Text), second.Owner.Describe())
}

func (snapshot *Snapshot) ShellLandmarks() ShellLandmarks {
	landmarks := ShellLandmarks{}
	for _, element := range snapshot.Elements {
		switch landmarkRole(element) {
		case "main":
			landmarks.Main = firstElement(landmarks.Main, element)
		case "navigation":
			landmarks.Rail = snapshot.railCandidate(landmarks.Rail, element)
		case "complementary":
			snapshot.placeComplementary(&landmarks, element)
		}
	}
	return landmarks
}

func landmarkRole(element *Element) string {
	if role := element.Attributes["role"]; role != "" {
		return role
	}
	return map[string]string{"main": "main", "nav": "navigation", "aside": "complementary"}[element.Tag]
}

func firstElement(current *Element, candidate *Element) *Element {
	if current != nil {
		return current
	}
	return candidate
}

func (snapshot *Snapshot) railCandidate(current *Element, candidate *Element) *Element {
	if current != nil || !strings.EqualFold(strings.TrimSpace(snapshot.AccessibleName(candidate)), "Domains") {
		return current
	}
	return candidate
}

func (snapshot *Snapshot) placeComplementary(landmarks *ShellLandmarks, element *Element) {
	isThread := element.Attributes["id"] == "thread" || strings.Contains(strings.ToLower(snapshot.AccessibleName(element)), "thread")
	if isThread {
		landmarks.Thread = firstElement(landmarks.Thread, element)
		return
	}
	landmarks.Sidebar = firstElement(landmarks.Sidebar, element)
}

func CheckShellLayout(snapshot *Snapshot, width int64) []string {
	landmarks := snapshot.ShellLandmarks()
	findings := missingLandmarks(landmarks)
	if len(findings) > 0 {
		return findings
	}
	if width < narrowLayoutLimit {
		return snapshot.narrowLayoutFindings(landmarks)
	}
	return snapshot.wideLayoutFindings(landmarks, width)
}

func missingLandmarks(landmarks ShellLandmarks) []string {
	findings := []string{}
	if landmarks.Main == nil {
		findings = append(findings, "no main landmark (<main>) in the shell")
	}
	if landmarks.Rail == nil {
		findings = append(findings, `no Domains rail (nav labelled "Domains") in the shell`)
	}
	if landmarks.Sidebar == nil {
		findings = append(findings, "no sidebar (an <aside>/complementary landmark that is not the thread) in the shell")
	}
	return findings
}

func (snapshot *Snapshot) narrowLayoutFindings(landmarks ShellLandmarks) []string {
	findings := []string{}
	visiblePanes := []string{}
	for _, pane := range []*Element{landmarks.Sidebar, landmarks.Main, landmarks.Thread} {
		if pane != nil && snapshot.IsVisible(pane) {
			visiblePanes = append(visiblePanes, snapshot.describeWithName(pane))
		}
	}
	if len(visiblePanes) != 1 {
		findings = append(findings, fmt.Sprintf("AC-UI-1: under %d px exactly one pane must show, found %d: %s", narrowLayoutLimit, len(visiblePanes), strings.Join(visiblePanes, ", ")))
	}
	if snapshot.IsVisible(landmarks.Rail) {
		findings = append(findings, fmt.Sprintf("AC-UI-1: the Domains rail %s must be hidden under %d px", snapshot.describeWithName(landmarks.Rail), narrowLayoutLimit))
	}
	return findings
}

func (snapshot *Snapshot) wideLayoutFindings(landmarks ShellLandmarks, width int64) []string {
	findings := []string{}
	columns := []*Element{landmarks.Rail, landmarks.Sidebar, landmarks.Main}
	for _, column := range columns {
		if !snapshot.IsVisible(column) {
			findings = append(findings, fmt.Sprintf("AC-UI-1: %s must be visible from %d px", snapshot.describeWithName(column), narrowLayoutLimit))
		}
	}
	findings = append(findings, overlappingColumns(snapshot, columns)...)
	if landmarks.Thread != nil && snapshot.IsVisible(landmarks.Thread) {
		findings = append(findings, fmt.Sprintf("AC-UI-1: the thread column %s shows at %d px while no thread is open", snapshot.describeWithName(landmarks.Thread), width))
	}
	return findings
}

func overlappingColumns(snapshot *Snapshot, columns []*Element) []string {
	findings := []string{}
	for first := range columns {
		for second := first + 1; second < len(columns); second++ {
			area := OverlapArea(columns[first].Bounds, columns[second].Bounds)
			if area > overlapTolerance {
				findings = append(findings, fmt.Sprintf("AC-UI-1: %s and %s overlap (%.0f px²); they must sit side by side",
					snapshot.describeWithName(columns[first]), snapshot.describeWithName(columns[second]), area))
			}
		}
	}
	return findings
}

func CheckReducedMotion(snapshot *Snapshot) []string {
	findings := []string{}
	for _, element := range snapshot.Elements {
		if !snapshot.IsVisible(element) {
			continue
		}
		transition := LongestDuration(element.Style["transition-duration"])
		animation := LongestDuration(element.Style["animation-duration"])
		if element.Style["transition-property"] != "none" && transition > motionTolerance {
			findings = append(findings, fmt.Sprintf("%s keeps a %.2fs transition (%s) under prefers-reduced-motion: reduce", element.Describe(), transition, element.Style["transition-property"]))
		}
		if element.Style["animation-name"] != "none" && animation > motionTolerance {
			findings = append(findings, fmt.Sprintf("%s keeps a %.2fs animation (%s) under prefers-reduced-motion: reduce", element.Describe(), animation, element.Style["animation-name"]))
		}
	}
	return findings
}

func CheckDarkBackground(snapshot *Snapshot) []string {
	for _, element := range []*Element{snapshot.Body(), snapshot.Document()} {
		if element == nil {
			continue
		}
		color := ParseColor(element.Style["background-color"])
		if color.Known && color.Alpha > 0.5 {
			return darkColorFinding(element, color)
		}
	}
	scheme := snapshot.Document().Style["color-scheme"]
	if strings.Contains(scheme, "dark") {
		return nil
	}
	return []string{fmt.Sprintf("prefers-color-scheme: dark: <body> and <html> have no background color and color-scheme is %q, so the page stays white", scheme)}
}

func darkColorFinding(element *Element, color CssColor) []string {
	if color.Luminance < darkLuminanceLimit {
		return nil
	}
	return []string{fmt.Sprintf("prefers-color-scheme: dark: <%s> background %s is light (relative luminance %.2f, needs < %.2f)",
		element.Tag, element.Style["background-color"], color.Luminance, darkLuminanceLimit)}
}

func CheckRailOnRight(snapshot *Snapshot) []string {
	rail := snapshot.ShellLandmarks().Rail
	if rail == nil || !snapshot.IsVisible(rail) {
		return []string{"right-to-left: the Domains rail is missing or hidden"}
	}
	center := rail.Bounds.X + rail.Bounds.Width/2
	if center > snapshot.ViewportWidth/2 {
		return nil
	}
	return []string{fmt.Sprintf("right-to-left: the Domains rail %s sits at x=%.0f..%.0f; it must be on the right side", snapshot.describeWithName(rail), rail.Bounds.X, rail.Bounds.X+rail.Bounds.Width)}
}

func CheckSafeAreaBands(snapshot *Snapshot, top float64, bottom float64) []string {
	findings := []string{}
	for _, element := range snapshot.Elements {
		if (element.DirectText == "" && !element.IsInteractive()) || !snapshot.IsVisible(element) {
			continue
		}
		if element.Bounds.Y < top-0.5 {
			findings = append(findings, fmt.Sprintf("%s starts at y=%.0f, inside the top safe area (%.0f px); pad it with env(safe-area-inset-top)", snapshot.describeWithName(element), element.Bounds.Y, top))
		}
		if snapshot.inBottomBand(element, bottom) {
			findings = append(findings, fmt.Sprintf("%s ends at y=%.0f, inside the bottom safe area (from %.0f px); pad it with env(safe-area-inset-bottom)",
				snapshot.describeWithName(element), element.Bounds.Y+element.Bounds.Height, snapshot.ViewportHeight-bottom))
		}
	}
	return findings
}

func (snapshot *Snapshot) inBottomBand(element *Element, bottom float64) bool {
	end := element.Bounds.Y + element.Bounds.Height
	if end <= snapshot.ViewportHeight-bottom+0.5 || element.Bounds.Y >= snapshot.ViewportHeight {
		return false
	}
	return snapshot.pinnedToViewport(element)
}

func (snapshot *Snapshot) pinnedToViewport(element *Element) bool {
	for current := element; current != nil; current = current.Parent {
		position := current.Style["position"]
		if position == "fixed" || position == "sticky" {
			return true
		}
		if current.Tag != "html" && current.Tag != "body" && scrollsVertically(current) {
			return false
		}
	}
	return snapshot.ContentHeight <= snapshot.ViewportHeight+0.5
}

func scrollsVertically(element *Element) bool {
	overflow := element.Style["overflow-y"]
	return (overflow == "auto" || overflow == "scroll") && element.ScrollBox.Height > element.ClientBox.Height+1
}

func FocusIndicatorWidth(focused *Element) float64 {
	widest := 0.0
	for current := focused; current != nil; current = current.Parent {
		widest = max(widest, ownIndicatorWidth(current))
		if current.Tag == "label" || strings.HasPrefix(current.Tag, "w-") {
			break
		}
	}
	return widest
}

func ownIndicatorWidth(element *Element) float64 {
	widest := 0.0
	if element.Style["outline-style"] != "none" {
		widest = PixelValue(element.Style["outline-width"])
	}
	for _, side := range []string{"top", "bottom", "left", "right"} {
		if element.Style["border-"+side+"-style"] != "none" {
			widest = max(widest, PixelValue(element.Style["border-"+side+"-width"]))
		}
	}
	return widest
}

func PaddedForTopInset(snapshot *Snapshot, top float64) *Element {
	for _, element := range snapshot.Elements {
		if snapshot.IsVisible(element) && PixelValue(element.Style["padding-top"]) >= top-0.5 {
			return element
		}
	}
	return nil
}
