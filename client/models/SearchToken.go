// SearchToken is the field:value expression under the caret. Index and Length
// count bytes of the query; Index is -1 when the caret is outside every token.
type SearchToken struct {
	Token  string `json:"token"`
	Term   string `json:"term"`
	Index  int    `json:"index"`
	Length int    `json:"length"`
}
