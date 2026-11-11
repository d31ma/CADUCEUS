import (
	"cmp"
	"encoding/json"
	"os"
	"time"

	"mesail.dev/packages/outbox"
)

// @Delegate
// @NeedsAll()
type SettingsDelegate struct{}

func (SettingsDelegate) variable() string { return "YON_APPLICATION_JSON" }

func (SettingsDelegate) defaultMaxAgeMinutes() int { return 7200 }

func (SettingsDelegate) defaultFirstDelaySeconds() int { return 60 }

func (SettingsDelegate) defaultMaxDelaySeconds() int { return 3600 }

func (SettingsDelegate) defaultLeaseSeconds() int { return 300 }

func (SettingsDelegate) defaultTimeoutSeconds() int { return 30 }

func (SettingsDelegate) defaultHeloName() string { return "mail-out.localhost" }

func (delegate *SettingsDelegate) Settings() MailOutSettings {
	settings := delegate.read()
	settings.MaxAgeMinutes = cmp.Or(settings.MaxAgeMinutes, delegate.defaultMaxAgeMinutes())
	settings.FirstDelaySeconds = cmp.Or(settings.FirstDelaySeconds, delegate.defaultFirstDelaySeconds())
	settings.MaxDelaySeconds = cmp.Or(settings.MaxDelaySeconds, delegate.defaultMaxDelaySeconds())
	settings.LeaseSeconds = cmp.Or(settings.LeaseSeconds, delegate.defaultLeaseSeconds())
	settings.TimeoutSeconds = cmp.Or(settings.TimeoutSeconds, delegate.defaultTimeoutSeconds())
	settings.HeloName = cmp.Or(settings.HeloName, delegate.defaultHeloName())
	return settings
}

func (delegate *SettingsDelegate) MaxAge() time.Duration {
	return time.Duration(delegate.Settings().MaxAgeMinutes) * time.Minute
}

func (delegate *SettingsDelegate) FirstDelay() time.Duration {
	return time.Duration(delegate.Settings().FirstDelaySeconds) * time.Second
}

func (delegate *SettingsDelegate) MaxDelay() time.Duration {
	return time.Duration(delegate.Settings().MaxDelaySeconds) * time.Second
}

func (delegate *SettingsDelegate) Lease() time.Duration {
	return time.Duration(delegate.Settings().LeaseSeconds) * time.Second
}

func (delegate *SettingsDelegate) Timeout() time.Duration {
	return time.Duration(delegate.Settings().TimeoutSeconds) * time.Second
}

func (delegate *SettingsDelegate) Outbox() (outbox.Settings, error) {
	return outbox.SettingsReader{}.FromEnvironment()
}

func (delegate *SettingsDelegate) read() MailOutSettings {
	snapshot := ApplicationSnapshot{}
	json.Unmarshal([]byte(os.Getenv(delegate.variable())), &snapshot)
	return snapshot.Yon.MailOut
}
