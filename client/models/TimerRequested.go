type TimerRequested struct {
	Name    string `json:"name"`
	DelayMs int    `json:"delayMs"`
	Repeat  bool   `json:"repeat"`
}
