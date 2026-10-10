// SearchKeyPressed is ArrowDown, ArrowUp, Escape or Enter in the search box,
// with the box's value and caret (in UTF-16 units) at that moment.
type SearchKeyPressed struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Caret int    `json:"caret"`
}
