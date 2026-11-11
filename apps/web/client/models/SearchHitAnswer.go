type SearchHitAnswer struct {
	Rows       []SearchHitRow `json:"rows"`
	NextCursor string         `json:"nextCursor"`
	State      string         `json:"state"`
}
