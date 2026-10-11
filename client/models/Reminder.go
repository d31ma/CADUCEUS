type Reminder struct {
	ID            string `json:"id"`
	EmailID       string `json:"emailId"`
	NoticeEmailID string `json:"noticeEmailId"`
	Subject       string `json:"subject"`
	Recipient     string `json:"recipient"`
	RemindAt      string `json:"remindAt"`
	Status        string `json:"status"`
}
