// A right click on a row, at a viewport position already kept inside the window.
type ContextMenuRequested struct {
	EmailID  string  `json:"emailId"`
	ThreadID string  `json:"threadId"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}
