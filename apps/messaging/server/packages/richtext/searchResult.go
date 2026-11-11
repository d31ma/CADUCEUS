package richtext

type searchResult struct {
	from  int
	index int
}

func (result searchResult) covers(from int) bool {
	return from >= result.from && (result.index < 0 || result.index >= from)
}
