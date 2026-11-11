type JobReport struct {
	Domains    int  `json:"domains"`
	FannedOut  int  `json:"fannedOut"`
	Pushed     int  `json:"pushed"`
	Skipped    int  `json:"skipped"`
	Summarized int  `json:"summarized"`
	More       bool `json:"more"`
}
