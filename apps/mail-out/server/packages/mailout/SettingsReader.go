package mailout

import (
	"cmp"
	"encoding/json"
)

type SettingsReader struct {
	Environment func(string) string
}

func (SettingsReader) variable() string { return "YON_APPLICATION_JSON" }

func (reader SettingsReader) Read() Settings {
	var snapshot struct {
		Yon struct {
			MailOut Settings `json:"mailout"`
		} `json:"yon"`
	}
	_ = json.Unmarshal([]byte(reader.Environment(reader.variable())), &snapshot)
	return reader.withDefaults(snapshot.Yon.MailOut)
}

func (SettingsReader) withDefaults(settings Settings) Settings {
	settings.MaxAgeMinutes = cmp.Or(settings.MaxAgeMinutes, 7200)
	settings.FirstDelaySeconds = cmp.Or(settings.FirstDelaySeconds, 60)
	settings.MaxDelaySeconds = cmp.Or(settings.MaxDelaySeconds, 3600)
	settings.LeaseSeconds = cmp.Or(settings.LeaseSeconds, 300)
	settings.TimeoutSeconds = cmp.Or(settings.TimeoutSeconds, 30)
	settings.HeloName = cmp.Or(settings.HeloName, "mail-out.localhost")
	return settings
}
