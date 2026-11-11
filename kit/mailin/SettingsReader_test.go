package mailin

import "testing"

func TestOutboxSettingsComeFromTheYonSection(t *testing.T) {
	settings, err := SettingsReader{Environment: goodSettings()}.Outbox()
	check(t, err == nil, "settings missing")
	check(t, settings.Producer == "mail-in", "wrong producer")
	check(t, settings.Subscribers[0].Name == "messaging", "wrong subscriber")
	check(t, settings.Retry.MaxAttempts == 2, "wrong retry")
}

func TestOutboxSettingsRefuseText(t *testing.T) {
	_, err := SettingsReader{Environment: testEnvironment(map[string]string{"YON_APPLICATION_JSON": "not json"})}.Outbox()
	check(t, isUnavailable(err, "outbox settings"), "want outbox settings")
}

func TestOutboxSettingsRefuseAFileWithoutAnOutbox(t *testing.T) {
	_, err := SettingsReader{Environment: testEnvironment(map[string]string{"YON_APPLICATION_JSON": `{"yon":{}}`})}.Outbox()
	check(t, isUnavailable(err, "outbox settings"), "want outbox settings")
}

func TestTheServiceTokenComesFromTheEnvironment(t *testing.T) {
	token, err := SettingsReader{Environment: goodSettings()}.ServiceToken().Token()
	check(t, err == nil && token == "service-token", "wrong token")
}
