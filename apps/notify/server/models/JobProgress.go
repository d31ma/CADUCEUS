type JobProgress struct {
	Written int  `json:"written"`
	Pushed  int  `json:"pushed"`
	Skipped int  `json:"skipped"`
	Done    int  `json:"done"`
	More    bool `json:"more"`
}
