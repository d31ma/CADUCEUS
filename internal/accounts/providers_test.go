package accounts

import "testing"

func TestProviderCatalogCoversMailspringOnboardingChoices(t *testing.T) {
	want := []string{"gmail", "google-workspace", "office365", "yahoo", "outlook", "icloud", "fastmail", "gmx", "yandex", "imap"}
	providers := ProviderCatalog()
	if len(providers) != len(want) {
		t.Fatalf("ProviderCatalog() count = %d, want %d", len(providers), len(want))
	}
	for index, id := range want {
		if providers[index].ID != id {
			t.Fatalf("ProviderCatalog()[%d].ID = %q, want %q", index, providers[index].ID, id)
		}
	}
	if providers[3].Preset == nil || providers[3].Preset.IMAP.Host != "imap.mail.yahoo.com" {
		t.Fatalf("Yahoo preset = %#v", providers[3].Preset)
	}
	if providers[5].Guidance != GuidanceAppPassword {
		t.Fatalf("iCloud guidance = %q", providers[5].Guidance)
	}
	if providers[9].Preset != nil || providers[9].Authentication != AuthenticationPassword {
		t.Fatalf("generic IMAP choice = %#v", providers[9])
	}
}

func TestSecurityForStandardPortsMatchesManualOnboarding(t *testing.T) {
	tests := []struct {
		protocol string
		port     uint16
		want     SecurityMode
	}{
		{"imap", 143, SecurityNone}, {"imap", 993, SecurityTLS},
		{"smtp", 25, SecurityNone}, {"smtp", 465, SecurityTLS}, {"smtp", 587, SecurityStartTLS},
	}
	for _, test := range tests {
		if got, ok := SecurityForStandardPort(test.protocol, test.port); !ok || got != test.want {
			t.Fatalf("SecurityForStandardPort(%q, %d) = %q, %t; want %q", test.protocol, test.port, got, ok, test.want)
		}
	}
	if _, ok := SecurityForStandardPort("smtp", 2525); ok {
		t.Fatal("custom port unexpectedly selected a security mode")
	}
}

func TestProviderCatalogReportsOAuthAvailabilityPerProvider(t *testing.T) {
	providers := ProviderCatalogWithOAuth(true, false)
	if !providers[2].Configured || providers[0].Configured || providers[1].Configured {
		t.Fatalf("Microsoft-only catalog = %#v", providers[:3])
	}
	providers = ProviderCatalogWithOAuth(false, true)
	if providers[2].Configured || !providers[0].Configured || !providers[1].Configured {
		t.Fatalf("Google-only catalog = %#v", providers[:3])
	}
}
