type MessagePage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"nextCursor"`
}
