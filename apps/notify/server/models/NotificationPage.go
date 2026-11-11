type NotificationPage struct {
	Items      []Notification `json:"items"`
	NextCursor string         `json:"nextCursor"`
}
