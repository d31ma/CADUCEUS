type HtmlNode struct {
	Kind       string          `json:"kind"`
	Name       string          `json:"name"`
	Text       string          `json:"text"`
	Attributes []HtmlAttribute `json:"attributes"`
	Children   []*HtmlNode     `json:"children"`
	Parent     *HtmlNode       `json:"parent"`
}
