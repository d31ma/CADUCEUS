package startup

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func environmentOf(values map[string]string) Environment {
	return NewEnvironment(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}, func() []string {
		names := []string{}
		for name := range values {
			names = append(names, name)
		}
		return names
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
	if !strings.Contains(text, "MESAIL_DIRECTORY_URL") || !strings.Contains(text, "MESAIL_MAIL_IN_URL") || !strings.Contains(text, "MESAIL_RAW_STORE_DIR") || !strings.Contains(text, "MESAIL_SERVICETOKENS_URL") {
		t.Fatalf("unexpected error %q", text)
	}
}

func TestIdentityTokensNeedTheSecretFile(t *testing.T) {
	values := minimalValues()
	delete(values, "MESAIL_SERVICE_TOKEN")
	values["MESAIL_SERVICETOKENS_URL"] = "http://identity.local/api/v1/identity/service-token"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	if !errors.Is(err, MissingSetting{Name: "MESAIL_SERVICE_CLIENT_SECRET_FILE"}) {
		t.Fatalf("want the secret file to be required, got %v", err)
	}
}

func TestIdentityTokensUseTheRouteAndATemporaryCache(t *testing.T) {
	values := minimalValues()
	delete(values, "MESAIL_SERVICE_TOKEN")
	values["MESAIL_SERVICETOKENS_URL"] = "http://identity.local/api/v1/identity/service-token"
	values["MESAIL_SERVICE_CLIENT_SECRET_FILE"] = "/run/secrets/service"
	settings, err := NewSettingsReader(environmentOf(values)).Load()
	if err != nil {
		t.Fatal(err)
	}
	tokens := settings.TokenSettings
	if tokens.Url != values["MESAIL_SERVICETOKENS_URL"] || tokens.SecretFile != "/run/secrets/service" || !strings.HasPrefix(tokens.CacheFolder, os.TempDir()) {
		t.Fatalf("unexpected token settings %+v", tokens)
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

func TestTheLocalFolderIsRequired(t *testing.T) {
	values := minimalValues()
	delete(values, "MESAIL_RAW_STORE_DIR")
	_, err := NewSettingsReader(environmentOf(values)).Load()
	var missing MissingSetting
	if !errors.As(err, &missing) || missing.Name != "MESAIL_RAW_STORE_DIR" {
		t.Fatalf("want the folder to be required, got %v", err)
	}
}

func TestEveryRemovedS3VariableStopsTheStart(t *testing.T) {
	prefix := SettingsReader{}.removedRawStorePrefix()
	values := minimalValues()
	values[prefix+"_BUCKET"] = "mesail-raw"
	values[prefix+"_ENDPOINT"] = "http://floci.local:4566"
	_, err := NewSettingsReader(environmentOf(values)).Load()
	var removed RemovedSetting
	if !errors.As(err, &removed) || removed.Name != prefix+"_BUCKET" {
		t.Fatalf("want the bucket variable to be named first, got %v", err)
	}
	if !strings.Contains(err.Error(), prefix+"_ENDPOINT was removed with the S3 raw store (ADR 0007)") || !strings.Contains(err.Error(), "MESAIL_RAW_STORE_DIR") {
		t.Fatalf("want every removed variable and the folder named, got %v", err)
	}
}

func TestTheProcessEnvironmentListsItsNames(t *testing.T) {
	t.Setenv("MESAIL_OP6_PROBE_NAME", "1")
	names := NewProcessEnvironment().NamesStartingWith("MESAIL_OP6_PROBE")
	if len(names) != 1 || names[0] != "MESAIL_OP6_PROBE_NAME" {
		t.Fatalf("unexpected names %v", names)
	}
}
