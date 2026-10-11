type Notice struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Priority    int            `json:"priority"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	Dismissable bool           `json:"dismissable"`
	Actions     []NoticeAction `json:"actions"`
	Classes     string         `json:"classes"`
}
