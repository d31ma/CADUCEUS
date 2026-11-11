type SearchPage struct {
	Items      []SearchHit `json:"items"`
	NextCursor string      `json:"nextCursor"`
}
