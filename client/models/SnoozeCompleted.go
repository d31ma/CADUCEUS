type SnoozeCompleted struct {
	SnoozeId   string `json:"snoozeId"`
	Count      int    `json:"count"`
	Until      string `json:"until"`
	Standalone bool   `json:"standalone"`
}
