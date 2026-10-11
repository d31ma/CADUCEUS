type ImportProgressed struct {
	Imported int    `json:"imported"`
	Total    int    `json:"total"`
	Current  string `json:"current"`
}
