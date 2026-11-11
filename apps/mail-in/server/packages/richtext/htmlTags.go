package richtext

type htmlTags struct{}

func (htmlTags) dropped() []string {
	return []string{
		"script", "style", "form", "iframe", "object", "embed", "svg", "math", "head", "title", "template",
		"noscript", "textarea", "select", "button", "frame", "frameset", "applet", "canvas", "audio", "video",
		"noembed", "noframes",
	}
}

func (htmlTags) boundaries() []string {
	return []string{
		"p", "div", "table", "thead", "tbody", "tfoot", "caption", "section", "article", "header", "footer",
		"main", "nav", "aside", "center", "address", "figure", "figcaption", "details", "summary", "dl", "dt",
		"dd", "fieldset", "legend", "hgroup", "body", "html",
	}
}

func (htmlTags) headings() []string {
	return []string{"h1", "h2", "h3", "h4", "h5", "h6"}
}

func (htmlTags) bold() []string {
	return []string{"b", "strong"}
}

func (htmlTags) italic() []string {
	return []string{"i", "em", "cite", "dfn", "var"}
}

func (htmlTags) strike() []string {
	return []string{"s", "strike", "del"}
}

func (htmlTags) code() []string {
	return []string{"code", "kbd", "samp", "tt"}
}

func (htmlTags) cells() []string {
	return []string{"td", "th"}
}

func (htmlTags) lineBreak() string {
	return "br"
}
