type HtmlParseState struct {
	Source   string      `json:"source"`
	Position int         `json:"position"`
	Body     *HtmlNode   `json:"body"`
	Open     []*HtmlNode `json:"open"`
	InBody   bool        `json:"inBody"`
}
