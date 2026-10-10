// FyloPage is one page of a paged FYLO query: its documents in FYLO's order
// and the cursor of the next page, or "" after the last page.
type FyloPage struct {
	Documents  []FyloDocument
	NextCursor string
}
