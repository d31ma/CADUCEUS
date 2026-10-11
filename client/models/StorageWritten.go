type StorageWritten struct {
	Area    string            `json:"area"`
	Values  map[string]string `json:"values"`
	Removed []string          `json:"removed"`
}
