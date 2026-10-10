type TimerRequested struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Key     string `json:"key"`
	DelayMs int    `json:"delayMs"`
}
