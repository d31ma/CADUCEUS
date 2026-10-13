type ToastRequested struct {
	Message     string `json:"message"`
	ActionLabel string `json:"actionLabel"`
	Action      string `json:"action"`
	Subject     string `json:"subject"`
	DurationMs  int    `json:"durationMs"`
}
