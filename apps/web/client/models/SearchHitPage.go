type SearchHitPage struct {
	Items      []SearchHit `json:"items"`
	NextCursor string      `json:"nextCursor"`
}
