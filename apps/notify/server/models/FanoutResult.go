import "mesail.dev/packages/notify"

type FanoutResult struct {
	Inserted  []NotificationRecord `json:"inserted"`
	Remaining []notify.Recipient   `json:"remaining"`
	Written   int                  `json:"written"`
}
