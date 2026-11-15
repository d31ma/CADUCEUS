package backups

import "mesail.dev/packages/attachments"

type Settings struct {
	Link  attachments.LinkSettings  `json:"link"`
	Limit attachments.LimitSettings `json:"limit"`
}
