type ThreadMessagePage struct {
	Items      []ThreadMessage `json:"items"`
	NextCursor string          `json:"nextCursor"`
}
