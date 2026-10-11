type ReminderRow struct {
	ID             string `json:"id"`
	EmailID        string `json:"emailId"`
	NoticeEmailID  string `json:"noticeEmailId"`
	Subject        string `json:"subject"`
	RecipientLabel string `json:"recipientLabel"`
	RemindAtLabel  string `json:"remindAtLabel"`
	Status         string `json:"status"`
}
