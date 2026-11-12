package startup

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func environmentOf(values map[string]string) Environment {
	return NewEnvironment(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
}

func minimalValues() map[string]string {
	return map[string]string{
		"MESAIL_DIRECTORY_URL": "http://directory.local",
		"MESAIL_MAIL_IN_URL":   "http://mail-in.local",
		"MESAIL_RAW_STORE_DIR": "/var/lib/mesail/raw",
		"MESAIL_SERVICE_TOKEN": "service-token",
		"MAIL_HOSTNAME":        "mx.mesail.test",
	}
}

func TestDefaultsMatchTheOldReceiver(t *testing.T) {
	settings, err := NewSettingsReader(environmentOf(minimalValues())).Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Address != "0.0.0.0:25" || settings.MaxSize != 26214400 || settings.MaxConnections != 200 || !settings.VerifyAuth || settings.Hostname != "mx.mesail.test" {
		t.Fatalf("unexpected defaults %+v", settings)
	}
}

func TestHostnameFallsBackToTheMachineName(t *testing.T) {
	values := minimalValues()
	delete(values, "MAIL_HOSTNAME")
	settings, _ := NewSettingsReader(environmentOf(values)).Load()
	if settings.Hostname == "" {
		t.Fatal("no hostname")
	}
}

func TestValuesFromTheEnvironmentAreUsed(t *testing.T) {
	values := minimalValues()
	values["SMTP_INBOUND_HOST"] = "127.0.0.1"
	values["SMTP_INBOUND_PORT"] = "2525"
	values["SMTP_MAX_SIZE"] = "1000"
	values["SMTP_MAX_CONNECTIONS"] = "5"
	values["MAIL_AUTH_VERIFY"] = "OFF"
	settings, err := NewSettingsReader(environmentOf(values)).Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Address != "127.0.0.1:2525" || settings.MaxSize != 1000 || settings.MaxConnections != 5 || settings.VerifyAuth {
		t.Fatalf("unexpected settings %+v", settings)
	}
}

func TestMissingRequiredValuesAreNamed(t *testing.T) {
	_, err := NewSettingsReader(environmentOf(map[string]string{})).Load()
	text := err.Error()
	if !strings.Contains(text, "MESAIL_DIRECTORY_URL") || !strings.Contains(text, "MESAIL_MAIL_IN_URL") || !strings.Contains(text, "MESAIL_RAW_STORE_DIR") || !strings.Contains(text, "MESAIL_SERVICE_TOKEN_URL") {
		t.Fatalf("unexpected error %q", text)
	}
}

func TestClientCredentialsNeedAllThreeValues(t *testing.T) {
	values := minimalValues()
	delete(values, "MESAIL_SERVICE_TOKEN")
	values["MESAIL_SERVICE_TOKEN_URL"] = "http://keycloak.local/token"
	values["MESAIL_SERVICE_CLIENT_ID"] = "smtp-edge"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	if !errors.Is(err, MissingSetting{Name: "MESAIL_SERVICE_CLIENT_SECRET_FILE"}) {
		t.Fatalf("want the secret file to be required, got %v", err)
	}
}

func TestBadNumberIsRefused(t *testing.T) {
	values := minimalValues()
	values["SMTP_MAX_SIZE"] = "-5"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	if err == nil || !strings.Contains(err.Error(), "SMTP_MAX_SIZE") {
		t.Fatalf("want an SMTP_MAX_SIZE error, got %v", err)
	}
}

func TestCertificateWithoutKeyIsRefused(t *testing.T) {
	values := minimalValues()
	values["MAIL_TLS_CERT_FILE"] = "/etc/mesail/cert.pem"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	if err == nil || !strings.Contains(err.Error(), "MAIL_TLS_KEY_FILE") {
		t.Fatalf("want a pair error, got %v", err)
	}
}

func s3Values() map[string]string {
	values := minimalValues()
	delete(values, "MESAIL_RAW_STORE_DIR")
	values["MESAIL_RAW_STORE_S3_BUCKET"] = "mesail-raw"
	values["MESAIL_RAW_STORE_S3_REGION"] = "eu-west-1"
	values["MESAIL_RAW_STORE_S3_ACCESS_KEY_ID"] = "edge-key"
	values["MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE"] = "/run/secrets/raw-store"
	values["MESAIL_RAW_STORE_S3_ENDPOINT"] = "http://floci.local:4566"
	return values
}

func TestTheLocalFolderIsUsedWithoutABucket(t *testing.T) {
	settings, err := NewSettingsReader(environmentOf(minimalValues())).Load()
	if err != nil || settings.RawStoreFolder != "/var/lib/mesail/raw" || settings.RawStoreS3 != (S3RawStoreSettings{}) {
		t.Fatalf("unexpected settings %+v %v", settings, err)
	}
}

func TestABucketChoosesS3AndNeedsNoFolder(t *testing.T) {
	values := s3Values()
	settings, err := NewSettingsReader(environmentOf(values)).Load()
	want := S3RawStoreSettings{Endpoint: "http://floci.local:4566", Bucket: "mesail-raw", Region: "eu-west-1", AccessKeyId: "edge-key", SecretFile: "/run/secrets/raw-store", Timeout: 5 * time.Minute}
	if err != nil || settings.RawStoreS3 != want || settings.RawStoreFolder != "" {
		t.Fatalf("unexpected settings %+v %v", settings, err)
	}
}

func TestABucketNeedsTheOtherS3Values(t *testing.T) {
	values := minimalValues()
	values["MESAIL_RAW_STORE_S3_BUCKET"] = "mesail-raw"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	text := err.Error()
	if !strings.Contains(text, "MESAIL_RAW_STORE_S3_REGION") || !strings.Contains(text, "MESAIL_RAW_STORE_S3_ACCESS_KEY_ID") || !strings.Contains(text, "MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE") || !strings.Contains(text, "MESAIL_RAW_STORE_S3_ENDPOINT") {
		t.Fatalf("unexpected error %q", text)
	}
}

func TestTheS3TimeoutCanBeChanged(t *testing.T) {
	values := s3Values()
	values["MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS"] = "600"
	settings, err := NewSettingsReader(environmentOf(values)).Load()
	if err != nil || settings.RawStoreS3.Timeout != 10*time.Minute {
		t.Fatalf("unexpected timeout %v %v", settings.RawStoreS3.Timeout, err)
	}
}

func TestABadS3TimeoutIsRefused(t *testing.T) {
	values := s3Values()
	values["MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS"] = "0"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	var invalid InvalidSetting
	if !errors.As(err, &invalid) || invalid.Name != "MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS" {
		t.Fatalf("want the timeout to be named, got %v", err)
	}
}

func TestABucketWithoutAnEndpointIsRefused(t *testing.T) {
	values := s3Values()
	delete(values, "MESAIL_RAW_STORE_S3_ENDPOINT")
	_, err := NewSettingsReader(environmentOf(values)).Load()
	var missing MissingSetting
	if !errors.As(err, &missing) || missing.Name != "MESAIL_RAW_STORE_S3_ENDPOINT" {
		t.Fatalf("want the endpoint to be required, got %v", err)
	}
}
