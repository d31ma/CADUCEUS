package carddav

import (
	"context"
	"errors"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestValidatorComposesMailAndOptionalCardDAVCapabilities(t *testing.T) {
	discovery := &fakeDiscoverer{result: Discovery{AddressBooks: []AddressBook{{URL: "https://dav.example/books/default/"}}}}
	validator := NewValidator(fakeMailValidator{capabilities: accounts.Capabilities{MailRead: true, MailWrite: true, Send: true}}, discovery)
	request := accounts.ValidationRequest{Provider: accounts.ProviderIMAP, Credentials: accounts.Credentials{Username: "alice", Password: "secret"}, CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}
	capabilities, err := validator.Validate(context.Background(), request)
	if err != nil || !capabilities.MailRead || !capabilities.Send || !capabilities.Contacts || discovery.calls != 1 {
		t.Fatalf("capabilities=%#v calls=%d err=%v", capabilities, discovery.calls, err)
	}
	request.CardDAV = nil
	capabilities, err = validator.Validate(context.Background(), request)
	if err != nil || capabilities.Contacts || discovery.calls != 1 {
		t.Fatalf("mail-only capabilities=%#v calls=%d err=%v", capabilities, discovery.calls, err)
	}
}

func TestValidatorClassifiesCardDAVFailureWithoutDetail(t *testing.T) {
	validator := NewValidator(fakeMailValidator{capabilities: accounts.Capabilities{MailRead: true}}, &fakeDiscoverer{err: ErrAuthentication})
	_, err := validator.Validate(context.Background(), accounts.ValidationRequest{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}})
	if !accounts.IsValidationFailure(err, accounts.ValidationCardDAV, accounts.ValidationAuthentication) || errors.Is(err, ErrAuthentication) {
		t.Fatalf("error=%v", err)
	}
}

type fakeMailValidator struct {
	capabilities accounts.Capabilities
	err          error
}

func (validator fakeMailValidator) Validate(context.Context, accounts.ValidationRequest) (accounts.Capabilities, error) {
	return validator.capabilities, validator.err
}

type fakeDiscoverer struct {
	result Discovery
	err    error
	calls  int
}

func (discoverer *fakeDiscoverer) Discover(context.Context, *accounts.CardDAVEndpoint, accounts.Credentials) (Discovery, error) {
	discoverer.calls++
	return discoverer.result, discoverer.err
}
