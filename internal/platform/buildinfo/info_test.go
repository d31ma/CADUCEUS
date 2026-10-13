package buildinfo

import "testing"

func TestNewUsesStableDevelopmentDefaults(t *testing.T) {
	info := New("", "", "")
	if info.Name != "caduceus" || info.Version != "dev" {
		t.Fatalf("unexpected identity: %+v", info)
	}
	if info.Commit == "" || info.BuiltAt == "" || info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("metadata contains an empty field: %+v", info)
	}
}

func TestNewPreservesInjectedMetadata(t *testing.T) {
	info := New("26.30.07", "abc123", "2026-07-26T12:00:00Z")
	if info.Version != "26.30.07" || info.Commit != "abc123" || info.BuiltAt != "2026-07-26T12:00:00Z" {
		t.Fatalf("injected metadata changed: %+v", info)
	}
}
