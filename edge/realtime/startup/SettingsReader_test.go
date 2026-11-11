package startup_test

import (
	"errors"
	"slices"
	"testing"

	"mesail.dev/edge/realtime/startup"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func readerFor(values map[string]string) startup.SettingsReader {
	return startup.NewSettingsReader(startup.NewEnvironment(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}))
}

func TestSecretShorterThan32IsRefused(t *testing.T) {
	_, err := readerFor(map[string]string{"MESAIL_REALTIME_GATEWAY_SECRET": validSecret[:31]}).Load()
	shortSecret := startup.ShortSecret{}
	if !errors.As(err, &shortSecret) || shortSecret.Minimum != 32 {
		t.Fatalf("expected ShortSecret, got %v", err)
	}
}

func TestMissingSecretIsRefused(t *testing.T) {
	_, err := readerFor(map[string]string{}).Load()
	if err == nil || err.Error() != "MESAIL_REALTIME_GATEWAY_SECRET must be set and at least 32 characters long" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDefaultsApply(t *testing.T) {
	settings, err := readerFor(map[string]string{"MESAIL_REALTIME_GATEWAY_SECRET": validSecret}).Load()
	if err != nil {
		t.Fatal(err)
	}
	expected := startup.Settings{
		PublicAddress:     "127.0.0.1:8116",
		ManagementAddress: "127.0.0.1:8117",
		BackendUrl:        "http://127.0.0.1:8106",
		Secret:            validSecret,
	}
	if settings.PublicAddress != expected.PublicAddress || settings.ManagementAddress != expected.ManagementAddress ||
		settings.BackendUrl != expected.BackendUrl || settings.Secret != expected.Secret || len(settings.Origins) != 0 {
		t.Fatalf("settings %+v", settings)
	}
}

func TestEnvironmentOverridesDefaults(t *testing.T) {
	settings, err := readerFor(map[string]string{
		"MESAIL_REALTIME_GATEWAY_SECRET": validSecret,
		"MESAIL_REALTIME_LISTEN":         "0.0.0.0:9000",
		"MESAIL_REALTIME_MANAGE_LISTEN":  "10.0.0.1:9001",
		"MESAIL_REALTIME_BACKEND":        "http://backend:8106",
		"MESAIL_REALTIME_ORIGINS":        " https://a.example, https://b.example ,,",
	}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.PublicAddress != "0.0.0.0:9000" || settings.ManagementAddress != "10.0.0.1:9001" || settings.BackendUrl != "http://backend:8106" {
		t.Fatalf("settings %+v", settings)
	}
	if !slices.Equal(settings.Origins, []string{"https://a.example", "https://b.example"}) {
		t.Fatalf("origins %q", settings.Origins)
	}
}
