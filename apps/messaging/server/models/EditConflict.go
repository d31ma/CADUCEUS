type EditConflict struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Current Message `json:"current"`
}
