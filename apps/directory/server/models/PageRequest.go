type PageRequest struct {
	Collection string         `json:"collection"`
	Where      map[string]any `json:"where"`
	Cursor     string         `json:"cursor"`
	Size       int            `json:"size"`
}
