import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ContentProtectionUtils rewrites untrusted mail HTML before it reaches the
// sandboxed reader frame, as email-content-protection.js did: it removes
// active content and trackers, applies the remote-image policy and marks
// quoted replies and disguised links.
type ContentProtectionUtils struct{}

func (ContentProtectionUtils) BuildDocument(html string, options ProtectionOptions) ProtectedDocument {
	result := ContentProtectionUtils{}.protect(html, options)
	result.DocumentHtml = ContentProtectionUtils{}.document(result.Html, options)
	return result
}

func (ContentProtectionUtils) unsafeElements() []string {
	return []string{"script", "iframe", "frame", "frameset", "object", "embed", "applet", "form", "input", "button", "textarea", "select", "option", "link", "base", "meta"}
}

func (ContentProtectionUtils) remoteAttributes() []string {
	return []string{"src", "background", "poster", "placeholder", "icon"}
}

func (ContentProtectionUtils) quotedClasses() []string {
	return []string{"gmail_quote", "gmail_extra", "yahoo_quoted", "moz-cite-prefix"}
}

func (ContentProtectionUtils) pattern(name string) *regexp.Regexp {
	sources := map[string]string{
		"safe":   `(?i)^(?:https?:|mailto:|tel:|#|/)`,
		"url":    `(?i)url\(\s*['"]?([^'")]+)['"]?\s*\)`,
		"hidden": `(?i)(?:display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0(?:\D|$))`,
		"size":   `(?i)^\d+(?:\.\d+)?(?:px)?$`,
		"spaces": `\s+`,
	}
	return regexp.MustCompile(sources[name])
}

func (ContentProtectionUtils) protect(html string, options ProtectionOptions) ProtectedDocument {
	body := HtmlDocumentUtils{}.Parse(html)
	result := &ProtectedDocument{RemoteUrls: []string{}}
	ContentProtectionUtils{}.removeUnsafe(body)
	ContentProtectionUtils{}.protectStyles(body, options, result)
	ContentProtectionUtils{}.protectElements(body, options, result)
	result.QuotedCount = ContentProtectionUtils{}.markQuoted(body)
	result.Html = HtmlDocumentUtils{}.Serialize(body)
	return *result
}

func (ContentProtectionUtils) removeUnsafe(body *HtmlNode) {
	for _, element := range (HtmlDocumentUtils{}).Elements(body) {
		ContentProtectionUtils{}.removeIfUnsafe(element)
	}
}

func (ContentProtectionUtils) removeIfUnsafe(element *HtmlNode) {
	if slices.Contains(ContentProtectionUtils{}.unsafeElements(), element.Name) {
		HtmlDocumentUtils{}.Remove(element)
	}
}

func (ContentProtectionUtils) protectStyles(body *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	for _, element := range (HtmlDocumentUtils{}).Elements(body) {
		ContentProtectionUtils{}.protectStyleElement(element, options, result)
	}
}

func (ContentProtectionUtils) styleName() string { return "style" }

func (ContentProtectionUtils) protectStyleElement(element *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	if element.Name != (ContentProtectionUtils{}).styleName() {
		return
	}
	protected := StyleProtectionUtils{}.ProtectCss(HtmlDocumentUtils{}.TextContent(element), options)
	ContentProtectionUtils{}.count(result, protected)
	ContentProtectionUtils{}.replaceStyle(element, protected.Css)
}

func (ContentProtectionUtils) count(result *ProtectedDocument, protected ProtectedCss) {
	result.RemoteCount += protected.RemoteCount
	result.BlockedCount += protected.BlockedCount
	result.TrackingCount += protected.TrackingCount
	for _, url := range protected.RemoteUrls {
		result.RemoteUrls = StyleProtectionUtils{}.RememberUrl(result.RemoteUrls, url)
	}
}

func (ContentProtectionUtils) replaceStyle(element *HtmlNode, css string) {
	if strings.TrimSpace(css) == "" {
		HtmlDocumentUtils{}.Remove(element)
		return
	}
	HtmlDocumentUtils{}.ReplaceText(element, css)
}

func (ContentProtectionUtils) protectElements(body *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	for _, element := range (HtmlDocumentUtils{}).Elements(body) {
		ContentProtectionUtils{}.protectElement(element, options, result)
	}
}

func (ContentProtectionUtils) protectElement(element *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	element.Attributes = slices.DeleteFunc(element.Attributes, func(attribute HtmlAttribute) bool { return ContentProtectionUtils{}.unsafeAttribute(attribute.Name) })
	ContentProtectionUtils{}.checkHref(element)
	ContentProtectionUtils{}.markLink(element, result)
	style, _ := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.styleName())
	candidates := ContentProtectionUtils{}.candidateUrls(element, style)
	if (ContentProtectionUtils{}).isTracker(element, style, candidates) {
		ContentProtectionUtils{}.removeTracker(element, candidates, result)
		return
	}
	ContentProtectionUtils{}.protectRemoteAttributes(element, options, result)
	ContentProtectionUtils{}.protectSourceSet(element, options, result)
	ContentProtectionUtils{}.protectStyleAttribute(element, style, options, result)
}

func (ContentProtectionUtils) unsafeAttribute(name string) bool {
	if strings.HasPrefix(name, "on") || name == "srcdoc" || name == "formaction" || strings.HasPrefix(name, "data-caduceus-") {
		return true
	}
	return false
}

func (ContentProtectionUtils) linkHref() string { return "href" }

func (ContentProtectionUtils) checkHref(element *HtmlNode) {
	href, found := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.linkHref())
	safe := ContentProtectionUtils{}.pattern("safe").MatchString(strings.TrimSpace(href))
	if found && href != "" && !safe {
		HtmlDocumentUtils{}.RemoveAttribute(element, ContentProtectionUtils{}.linkHref())
	}
}

func (ContentProtectionUtils) markLink(element *HtmlNode, result *ProtectedDocument) {
	href, found := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.linkHref())
	if element.Name != "a" || !found {
		return
	}
	HtmlDocumentUtils{}.SetAttribute(element, "target", "_blank")
	HtmlDocumentUtils{}.SetAttribute(element, "rel", "noopener noreferrer nofollow")
	ContentProtectionUtils{}.warnLink(element, LinkDestinationUtils{}.Analyze(href, HtmlDocumentUtils{}.TextContent(element)), result)
}

func (ContentProtectionUtils) classAttribute() string { return "class" }

func (ContentProtectionUtils) warnLink(element *HtmlNode, destination LinkDestination, result *ProtectedDocument) {
	if !destination.Suspicious {
		return
	}
	result.SuspiciousLinkCount++
	HtmlDocumentUtils{}.SetAttribute(element, ContentProtectionUtils{}.classAttribute(), ContentProtectionUtils{}.withClass(element, "caduceus-suspicious-link"))
	HtmlDocumentUtils{}.SetAttribute(element, "data-caduceus-link-warning", destination.Reason)
	HtmlDocumentUtils{}.SetAttribute(element, "data-caduceus-link-target", destination.Target)
	HtmlDocumentUtils{}.SetAttribute(element, "data-caduceus-link-host", destination.DestinationHost)
	HtmlDocumentUtils{}.SetAttribute(element, "data-caduceus-link-label-host", destination.DisplayedHost)
	HtmlDocumentUtils{}.SetAttribute(element, "title", "Suspicious link: "+destination.Reason)
}

func (ContentProtectionUtils) space() string { return " " }

func (ContentProtectionUtils) withClass(element *HtmlNode, name string) string {
	current, _ := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.classAttribute())
	classes := strings.Fields(current)
	if slices.Contains(classes, name) {
		return strings.Join(classes, ContentProtectionUtils{}.space())
	}
	return strings.Join(append(classes, name), ContentProtectionUtils{}.space())
}

func (ContentProtectionUtils) candidateUrls(element *HtmlNode, style string) []string {
	urls := ContentProtectionUtils{}.attributeUrls(element)
	urls = append(urls, ContentProtectionUtils{}.sourceSetUrls(element)...)
	return append(urls, ContentProtectionUtils{}.styleUrls(style)...)
}

func (ContentProtectionUtils) attributeUrls(element *HtmlNode) []string {
	urls := []string{}
	for _, name := range (ContentProtectionUtils{}).remoteAttributes() {
		value, _ := HtmlDocumentUtils{}.Attribute(element, name)
		urls = append(urls, ContentProtectionUtils{}.remoteOnly(value)...)
	}
	return urls
}

func (ContentProtectionUtils) remoteOnly(value string) []string {
	remote := StyleProtectionUtils{}.IsRemoteContentUrl(value)
	if value != "" && remote {
		return []string{value}
	}
	return []string{}
}

func (ContentProtectionUtils) sourceSetName() string { return "srcset" }

func (ContentProtectionUtils) comma() string { return "," }

func (ContentProtectionUtils) sourceSetUrls(element *HtmlNode) []string {
	sourceSet, _ := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.sourceSetName())
	urls := []string{}
	for _, entry := range strings.Split(sourceSet, (ContentProtectionUtils{}).comma()) {
		urls = append(urls, ContentProtectionUtils{}.remoteOnly(ContentProtectionUtils{}.entryUrl(entry))...)
	}
	return urls
}

func (ContentProtectionUtils) entryUrl(entry string) string {
	fields := strings.Fields(entry)
	return strings.Join(fields[:min(1, len(fields))], "")
}

func (ContentProtectionUtils) styleUrls(style string) []string {
	urls := []string{}
	for _, found := range (ContentProtectionUtils{}).pattern("url").FindAllStringSubmatch(style, -1) {
		urls = append(urls, ContentProtectionUtils{}.remoteOnly(found[1])...)
	}
	return urls
}

func (ContentProtectionUtils) isTracker(element *HtmlNode, style string, candidates []string) bool {
	utils := ContentProtectionUtils{}
	if slices.ContainsFunc(candidates, (StyleProtectionUtils{}).IsKnownTrackingUrl) || (len(candidates) > 0 && utils.isInvisible(element, style)) {
		return true
	}
	return false
}

func (ContentProtectionUtils) removeTracker(element *HtmlNode, candidates []string, result *ProtectedDocument) {
	result.RemoteCount += len(candidates)
	result.TrackingCount += max(len(candidates), 1)
	HtmlDocumentUtils{}.Remove(element)
}

func (ContentProtectionUtils) zero(present bool, size float64) bool {
	return present && size == 0
}

func (ContentProtectionUtils) pixelLimit() float64 { return 3 }

func (ContentProtectionUtils) tiny(width float64, height float64, hasWidth bool, hasHeight bool) bool {
	limit := ContentProtectionUtils{}.pixelLimit()
	if hasWidth && hasHeight && width <= limit && height <= limit {
		return true
	}
	return false
}

func (ContentProtectionUtils) widthName() string { return "width" }

func (ContentProtectionUtils) heightName() string { return "height" }

func (ContentProtectionUtils) isInvisible(element *HtmlNode, style string) bool {
	utils := ContentProtectionUtils{}
	widthAttribute, _ := HtmlDocumentUtils{}.Attribute(element, utils.widthName())
	heightAttribute, _ := HtmlDocumentUtils{}.Attribute(element, utils.heightName())
	width, hasWidth := utils.dimension(widthAttribute, style, utils.widthName())
	height, hasHeight := utils.dimension(heightAttribute, style, utils.heightName())
	hidden := utils.pattern("hidden").MatchString(style)
	zeroWidth := utils.zero(hasWidth, width)
	zeroHeight := utils.zero(hasHeight, height)
	tiny := utils.tiny(width, height, hasWidth, hasHeight)
	if zeroWidth || zeroHeight || tiny || hidden {
		return true
	}
	return false
}

func (ContentProtectionUtils) dimension(attribute string, style string, name string) (float64, bool) {
	raw := strings.TrimSpace(attribute)
	if (ContentProtectionUtils{}).pattern("size").MatchString(raw) {
		return ContentProtectionUtils{}.number(strings.TrimSuffix(strings.ToLower(raw), "px"))
	}
	return ContentProtectionUtils{}.styleDimension(style, name)
}

func (ContentProtectionUtils) styleDimension(style string, name string) (float64, bool) {
	found := regexp.MustCompile(`(?i)(?:^|;)\s*` + name + `\s*:\s*(\d+(?:\.\d+)?)px`).FindStringSubmatch(style)
	if found == nil {
		return 0, false
	}
	return ContentProtectionUtils{}.number(found[1])
}

func (ContentProtectionUtils) number(text string) (float64, bool) {
	value, err := strconv.ParseFloat(text, 64)
	return value, err == nil
}

func (ContentProtectionUtils) protectRemoteAttributes(element *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	for _, name := range (ContentProtectionUtils{}).remoteAttributes() {
		ContentProtectionUtils{}.protectRemoteAttribute(element, name, options, result)
	}
}

func (ContentProtectionUtils) protectRemoteAttribute(element *HtmlNode, name string, options ProtectionOptions, result *ProtectedDocument) {
	value, _ := HtmlDocumentUtils{}.Attribute(element, name)
	switch {
	case ContentProtectionUtils{}.isInlineReference(value):
		ContentProtectionUtils{}.replaceInline(element, name, options.InlineSources[AttachmentContentUtils{}.NormalizeContentReference(value)])
	case ContentProtectionUtils{}.isRemoteValue(value):
		result.BlockedCount += ContentProtectionUtils{}.replaceRemote(element, name, ContentProtectionUtils{}.remembered(value, result), options)
	}
}

func (ContentProtectionUtils) isInlineReference(value string) bool {
	return value != "" && AttachmentContentUtils{}.IsContentReference(value)
}

func (ContentProtectionUtils) isRemoteValue(value string) bool {
	return value != "" && StyleProtectionUtils{}.IsRemoteContentUrl(value)
}

func (ContentProtectionUtils) replaceInline(element *HtmlNode, name string, source string) {
	if (StyleProtectionUtils{}).IsSafeEmbeddedSource(source) {
		HtmlDocumentUtils{}.SetAttribute(element, name, source)
		return
	}
	HtmlDocumentUtils{}.RemoveAttribute(element, name)
}

func (ContentProtectionUtils) remembered(value string, result *ProtectedDocument) string {
	normalized := StyleProtectionUtils{}.NormalizeRemoteUrl(value)
	result.RemoteUrls = StyleProtectionUtils{}.RememberUrl(result.RemoteUrls, normalized)
	result.RemoteCount++
	return normalized
}

func (ContentProtectionUtils) replaceRemote(element *HtmlNode, name string, normalized string, options ProtectionOptions) int {
	source := options.RemoteSources[normalized]
	safe := StyleProtectionUtils{}.IsSafeEmbeddedSource(source)
	if !options.AllowRemote || !safe {
		HtmlDocumentUtils{}.RemoveAttribute(element, name)
		return 1
	}
	HtmlDocumentUtils{}.SetAttribute(element, name, source)
	return 0
}

func (ContentProtectionUtils) protectSourceSet(element *HtmlNode, options ProtectionOptions, result *ProtectedDocument) {
	sourceSet, found := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.sourceSetName())
	if !found || sourceSet == "" {
		return
	}
	inlineSafe := ContentProtectionUtils{}.inlineEntries(ContentProtectionUtils{}.entries(sourceSet), options)
	ContentProtectionUtils{}.writeSourceSet(element, ContentProtectionUtils{}.remoteEntries(inlineSafe, options, result))
}

func (ContentProtectionUtils) entries(sourceSet string) []string {
	found := []string{}
	for _, entry := range strings.Split(sourceSet, (ContentProtectionUtils{}).comma()) {
		found = append(found, ContentProtectionUtils{}.nonEmpty(strings.TrimSpace(entry))...)
	}
	return found
}

func (ContentProtectionUtils) nonEmpty(entry string) []string {
	if entry == "" {
		return []string{}
	}
	return []string{entry}
}

func (ContentProtectionUtils) inlineEntries(entries []string, options ProtectionOptions) []string {
	kept := []string{}
	for _, entry := range entries {
		kept = append(kept, ContentProtectionUtils{}.inlineEntry(entry, options)...)
	}
	return kept
}

func (ContentProtectionUtils) inlineEntry(entry string, options ProtectionOptions) []string {
	url, descriptor := ContentProtectionUtils{}.splitEntry(entry)
	if !(AttachmentContentUtils{}).IsContentReference(url) {
		return []string{entry}
	}
	return ContentProtectionUtils{}.safeEntry(options.InlineSources[AttachmentContentUtils{}.NormalizeContentReference(url)], descriptor)
}

func (ContentProtectionUtils) safeEntry(source string, descriptor string) []string {
	if !(StyleProtectionUtils{}).IsSafeEmbeddedSource(source) {
		return []string{}
	}
	return []string{strings.TrimSpace(source + ContentProtectionUtils{}.space() + descriptor)}
}

func (ContentProtectionUtils) splitEntry(entry string) (string, string) {
	fields := ContentProtectionUtils{}.pattern("spaces").Split(entry, -1)
	return fields[0], strings.Join(fields[1:], ContentProtectionUtils{}.space())
}

func (ContentProtectionUtils) remoteEntries(entries []string, options ProtectionOptions, result *ProtectedDocument) []string {
	kept := []string{}
	for _, entry := range entries {
		kept = append(kept, ContentProtectionUtils{}.remoteEntry(entry, options, result)...)
	}
	return kept
}

func (ContentProtectionUtils) remoteEntry(entry string, options ProtectionOptions, result *ProtectedDocument) []string {
	url, descriptor := ContentProtectionUtils{}.splitEntry(entry)
	if !(StyleProtectionUtils{}).IsRemoteContentUrl(url) {
		return []string{entry}
	}
	kept, blocked := ContentProtectionUtils{}.allowedEntry(options.RemoteSources[ContentProtectionUtils{}.remembered(url, result)], descriptor, options.AllowRemote)
	result.BlockedCount += blocked
	return kept
}

func (ContentProtectionUtils) allowedEntry(source string, descriptor string, allowRemote bool) ([]string, int) {
	safe := StyleProtectionUtils{}.IsSafeEmbeddedSource(source)
	if !allowRemote || !safe {
		return []string{}, 1
	}
	return []string{strings.TrimSpace(source + ContentProtectionUtils{}.space() + descriptor)}, 0
}

func (ContentProtectionUtils) writeSourceSet(element *HtmlNode, entries []string) {
	if len(entries) == 0 {
		HtmlDocumentUtils{}.RemoveAttribute(element, ContentProtectionUtils{}.sourceSetName())
		return
	}
	HtmlDocumentUtils{}.SetAttribute(element, ContentProtectionUtils{}.sourceSetName(), strings.Join(entries, ContentProtectionUtils{}.comma()+ContentProtectionUtils{}.space()))
}

func (ContentProtectionUtils) protectStyleAttribute(element *HtmlNode, style string, options ProtectionOptions, result *ProtectedDocument) {
	if style == "" {
		return
	}
	protected := StyleProtectionUtils{}.ProtectCss(style, options)
	ContentProtectionUtils{}.count(result, protected)
	ContentProtectionUtils{}.writeStyle(element, protected.Css)
}

func (ContentProtectionUtils) writeStyle(element *HtmlNode, css string) {
	if strings.TrimSpace(css) == "" {
		HtmlDocumentUtils{}.RemoveAttribute(element, ContentProtectionUtils{}.styleName())
		return
	}
	HtmlDocumentUtils{}.SetAttribute(element, ContentProtectionUtils{}.styleName(), css)
}

func (ContentProtectionUtils) markQuoted(body *HtmlNode) int {
	count := 0
	for _, element := range (HtmlDocumentUtils{}).Elements(body) {
		count += ContentProtectionUtils{}.markIfQuoted(element)
	}
	return count
}

func (ContentProtectionUtils) quotedMarker() string { return "data-caduceus-quoted" }

func (ContentProtectionUtils) markIfQuoted(element *HtmlNode) int {
	utils := ContentProtectionUtils{}
	if !utils.isQuoted(element) || utils.hasQuotedAncestor(element.Parent) {
		return 0
	}
	HtmlDocumentUtils{}.SetAttribute(element, ContentProtectionUtils{}.quotedMarker(), "")
	return 1
}

func (ContentProtectionUtils) isQuoted(element *HtmlNode) bool {
	classValue, _ := HtmlDocumentUtils{}.Attribute(element, ContentProtectionUtils{}.classAttribute())
	kind, _ := HtmlDocumentUtils{}.Attribute(element, "type")
	classes := strings.Fields(classValue)
	quotedClass := slices.ContainsFunc(classes, func(name string) bool { return slices.Contains(ContentProtectionUtils{}.quotedClasses(), name) })
	if element.Name == "blockquote" || kind == "cite" || quotedClass {
		return true
	}
	return false
}

func (ContentProtectionUtils) hasQuotedAncestor(node *HtmlNode) bool {
	ancestor := node
	for (ContentProtectionUtils{}).searching(ancestor) {
		ancestor = ancestor.Parent
	}
	return ancestor != nil
}

func (ContentProtectionUtils) searching(ancestor *HtmlNode) bool {
	return ancestor != nil && !ContentProtectionUtils{}.isQuoted(ancestor)
}

func (ContentProtectionUtils) document(body string, options ProtectionOptions) string {
	csp := "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:; media-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; script-src 'none'"
	scheme, background, foreground, link := ContentProtectionUtils{}.palette(options.ColorMode)
	return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="` + csp + `"><style>html{color-scheme:` + scheme + `;background:` + background + `;color:` + foreground + `}body{margin:0;padding:16px;font:14px/1.55 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;overflow-wrap:anywhere}a{color:` + link + `}img{max-width:100%;height:auto}table{max-width:100%}[data-caduceus-quoted]{display:none!important}body.show-quoted [data-caduceus-quoted]{display:block!important}.caduceus-suspicious-link{outline:2px solid #b3261e;outline-offset:2px}.caduceus-suspicious-link::after{content:' ⚠';color:#b3261e;font-weight:700}</style></head><body` + ContentProtectionUtils{}.quotedClass(options.ShowQuoted) + `>` + body + `</body></html>`
}

func (ContentProtectionUtils) palette(mode string) (string, string, string, string) {
	if mode == "dark" {
		return mode, "#181a1b", "#f2f2f2", "#8ab4f8"
	}
	return "light", "#fff", "#111", "#0645ad"
}

func (ContentProtectionUtils) quotedClass(show bool) string {
	if show {
		return ` class="show-quoted"`
	}
	return ""
}
