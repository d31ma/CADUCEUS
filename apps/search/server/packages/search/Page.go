package search

type Page struct {
	Items      []Hit  `json:"items"`
	NextCursor string `json:"nextCursor"`
}
