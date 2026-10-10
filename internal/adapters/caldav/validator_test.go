package caldav

import (
	"context"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

type fakeValidator struct{ capabilities accounts.Capabilities }

func (validator fakeValidator) Validate(context.Context, accounts.ValidationRequest) (accounts.Capabilities, error) {
	return validator.capabilities, nil
}

type fakeDiscoverer struct{ calls int }

func (discoverer *fakeDiscoverer) Discover(context.Context, *accounts.CalDAVEndpoint, accounts.Credentials) (Discovery, error) {
	discoverer.calls++
	return Discovery{Calendars: []Calendar{{URL: "https://dav.example/calendar/"}}}, nil
}

func TestValidatorAddsCalendarCapabilityOnlyWhenRequestedAndDiscovered(t *testing.T) {
	discoverer := &fakeDiscoverer{}
	validator := NewValidator(fakeValidator{capabilities: accounts.Capabilities{MailRead: true, Contacts: true}}, discoverer)
	request := accounts.ValidationRequest{CalDAV: &accounts.CalDAVEndpoint{URL: "https://dav.example/"}}
	capabilities, err := validator.Validate(context.Background(), request)
	if err != nil || !capabilities.MailRead || !capabilities.Contacts || !capabilities.Calendar || discoverer.calls != 1 {
		t.Fatalf("capabilities=%#v calls=%d err=%v", capabilities, discoverer.calls, err)
	}
}
