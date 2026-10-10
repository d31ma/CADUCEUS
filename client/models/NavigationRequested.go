type NavigationRequested struct {
	Target  string `json:"target"`
	Replace bool   `json:"replace"`
	Reload  bool   `json:"reload"`
}
