package devicebox

import "mesail.dev/packages/outbox"

type Settings struct {
	Outbox      outbox.Settings
	OutboxError error
}

func (settings Settings) Checked() error {
	if settings.OutboxError != nil {
		return Unavailable{}.OutboxSettingsMissing(settings.OutboxError)
	}
	return nil
}
