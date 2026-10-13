package accounts

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestDiscoveryUsesMXProviderBeforeAutoconfig(t *testing.T) {
	fetcher := &recordingAutoconfig{}
	service := NewDiscovery(fakeMX{hosts: []string{"aspmx.l.google.com"}}, fetcher)
	result, err := service.Discover(context.Background(), "person@company.example")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if result.ChoiceID != "google-workspace" || result.Source != DiscoveryMX || len(fetcher.urls) != 0 {
		t.Fatalf("Discover() = %#v fetches=%#v", result, fetcher.urls)
	}
}

func TestDiscoveryFallsThroughAutoconfigURLsAndExpandsUsername(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?><clientConfig><emailProvider id="example.com">
  <incomingServer type="imap"><hostname>mail.example.com</hostname><port>993</port><socketType>SSL</socketType><username>%EMAILADDRESS%</username></incomingServer>
  <outgoingServer type="smtp"><hostname>submit.example.com</hostname><port>587</port><socketType>STARTTLS</socketType><username>%EMAILLOCALPART%</username></outgoingServer>
</emailProvider></clientConfig>`)
	fetcher := &recordingAutoconfig{responses: map[string][]byte{
		"https://example.com/.well-known/autoconfig/mail/config-v1.1.xml?emailaddress=alice%40example.com": xml,
	}}
	service := NewDiscovery(fakeMX{}, fetcher)
	result, err := service.Discover(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if result.Source != DiscoveryAutoconfig || result.Connection.IMAP == nil || result.Connection.IMAP.Username != "alice@example.com" {
		t.Fatalf("Discover() = %#v", result)
	}
	if result.Connection.SMTP == nil || result.Connection.SMTP.Username != "alice" || result.Connection.SMTP.Security != SecurityStartTLS {
		t.Fatalf("SMTP discovery = %#v", result.Connection.SMTP)
	}
	if len(fetcher.urls) != 2 {
		t.Fatalf("autoconfig attempts = %#v", fetcher.urls)
	}
}

func TestDiscoveryUsesLegacyPresetThenConservativeFallback(t *testing.T) {
	service := NewDiscovery(fakeMX{}, &recordingAutoconfig{})
	legacy, err := service.Discover(context.Background(), "alice@fastmail.com")
	if err != nil || legacy.Source != DiscoveryPreset || legacy.Connection.IMAP.Host != "imap.fastmail.com" {
		t.Fatalf("legacy Discover() = %#v, %v", legacy, err)
	}
	fallback, err := service.Discover(context.Background(), "alice@unknown.example")
	if err != nil || fallback.Source != DiscoveryFallback || fallback.Connection.IMAP.Host != "imap.unknown.example" || fallback.Connection.SMTP.Port != 587 {
		t.Fatalf("fallback Discover() = %#v, %v", fallback, err)
	}
}

func TestDiscoveryRejectsMalformedAddress(t *testing.T) {
	service := NewDiscovery(fakeMX{}, &recordingAutoconfig{})
	if _, err := service.Discover(context.Background(), "not-an-address"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Discover() error = %v, want ErrInvalid", err)
	}
}

type fakeMX struct {
	hosts []string
}

func (resolver fakeMX) LookupMX(context.Context, string) ([]string, error) {
	return resolver.hosts, nil
}

type recordingAutoconfig struct {
	responses map[string][]byte
	urls      []string
}

func (fetcher *recordingAutoconfig) Fetch(_ context.Context, url string) ([]byte, error) {
	fetcher.urls = append(fetcher.urls, url)
	if response := fetcher.responses[url]; response != nil {
		return response, nil
	}
	return nil, fmt.Errorf("not found")
}
