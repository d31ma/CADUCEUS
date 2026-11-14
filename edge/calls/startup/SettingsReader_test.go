package startup

import (
	"errors"
	"slices"
	"testing"
)

func reader(values map[string]string) SettingsReader {
	return NewSettingsReader(NewEnvironment(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}))
}

func TestSettingsUseDefaults(t *testing.T) {
	settings, err := reader(map[string]string{"MESAIL_CALLS_KEYS_URL": "http://realtime:8106/api/v1/realtime/calls/keys"}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.HttpAddress != ":8120" || settings.MediaAddress != ":8121" || len(settings.PublicAddresses) != 0 {
		t.Fatalf("settings %+v", settings)
	}
}

func TestSettingsReadEveryValue(t *testing.T) {
	settings, err := reader(map[string]string{
		"MESAIL_CALLS_KEYS_URL":     "http://realtime/keys",
		"MESAIL_CALLS_LISTEN":       "127.0.0.1:9000",
		"MESAIL_CALLS_MEDIA_LISTEN": "0.0.0.0:9001",
		"MESAIL_CALLS_PUBLIC_IPS":   "203.0.113.7, 2001:db8::1",
	}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.HttpAddress != "127.0.0.1:9000" || settings.MediaAddress != "0.0.0.0:9001" || !slices.Equal(settings.PublicAddresses, []string{"203.0.113.7", "2001:db8::1"}) {
		t.Fatalf("settings %+v", settings)
	}
}

func TestSettingsNeedTheKeysUrl(t *testing.T) {
	_, err := reader(map[string]string{}).Load()
	missing := MissingSetting{}
	if !errors.As(err, &missing) || err.Error() != "MESAIL_CALLS_KEYS_URL must be set" {
		t.Fatalf("error %v", err)
	}
}

func TestSettingsRefuseANameAsPublicAddress(t *testing.T) {
	_, err := reader(map[string]string{"MESAIL_CALLS_KEYS_URL": "http://realtime/keys", "MESAIL_CALLS_PUBLIC_IPS": "calls.example.com"}).Load()
	invalid := InvalidAddress{}
	if !errors.As(err, &invalid) || invalid.Value != "calls.example.com" || err.Error() == "" {
		t.Fatalf("error %v", err)
	}
}
