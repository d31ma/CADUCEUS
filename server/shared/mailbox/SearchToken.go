// SearchToken is one lexical token of a search: text, a colon, a
// parenthesis or the end.
type SearchToken struct {
	Type  string
	Value string
}
