type FocusRequested struct {
	Selector string `json:"selector"`
	Remember bool   `json:"remember"`
	Restore  bool   `json:"restore"`
}
