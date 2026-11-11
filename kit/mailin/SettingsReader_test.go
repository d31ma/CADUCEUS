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

func TestMailInSettingsComeFromTheYonSection(t *testing.T) {
	application := `{"yon":{"mailin":{"directory":"http://directory.test","blobs":{"adapter":"s3","folder":"/blobs","link":{"seconds":60,"secret":"s"}}}}}`
	settings := SettingsReader{Environment: testEnvironment(map[string]string{"YON_APPLICATION_JSON": application, "AWS_REGION": "eu-west-1", "AWS_ENDPOINT_URL": "http://floci:4566"})}.MailIn()
	check(t, settings.Directory == "http://directory.test" && settings.Blobs.Adapter == "s3", "wrong settings")
	check(t, settings.Blobs.Region == "eu-west-1" && settings.Blobs.Endpoint == "http://floci:4566", "want the AWS environment")
	check(t, settings.Blobs.Link.Seconds == 60 && settings.Blobs.Bucket == "mesail-raw", "wrong link or bucket")
}

func TestMailInSettingsHaveDefaults(t *testing.T) {
	settings := SettingsReader{Environment: testEnvironment(nil)}.MailIn()
	check(t, settings.Blobs.Adapter == "local" && settings.Blobs.Region == "us-east-1", "wrong defaults")
	check(t, settings.Blobs.Link.Seconds == 300, "want five minutes")
}

func TestServiceTokensNeedTheKeycloakSettings(t *testing.T) {
	_, err := SettingsReader{Environment: goodSettings()}.Authorize()("example.com")
	check(t, err != nil, "want a refusal without keycloak settings")
}
