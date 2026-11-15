type EditRequest struct {
	Body      string `json:"body"`
	IfVersion int    `json:"ifVersion,omitempty"`
}
