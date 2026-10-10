type HtmlToken struct {
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Text        string          `json:"text"`
	Attributes  []HtmlAttribute `json:"attributes"`
	SelfClosing bool            `json:"selfClosing"`
}
