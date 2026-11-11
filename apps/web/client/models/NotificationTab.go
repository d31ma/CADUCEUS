type NotificationTab struct {
	Name        string `json:"name"`
	Id          string `json:"id"`
	Label       string `json:"label"`
	UnreadCount int    `json:"unreadCount"`
	HasUnread   bool   `json:"hasUnread"`
	Selected    bool   `json:"selected"`
}
