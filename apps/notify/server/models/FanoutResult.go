import "mesail.dev/packages/notifications"

type FanoutResult struct {
	Inserted  []NotificationRecord      `json:"inserted"`
	Remaining []notifications.Recipient `json:"remaining"`
	Written   int                       `json:"written"`
}
