package accounts

import (
	"context"
	"errors"
	"testing"
)

type capabilityBaseValidator struct {
	capabilities Capabilities
	err          error
}

func (validator capabilityBaseValidator) Validate(context.Context, ValidationRequest) (Capabilities, error) {
	return validator.capabilities, validator.err
}

func TestCapabilityValidatorAddsOnlyProviderCapabilities(t *testing.T) {
	validator := CapabilityValidator{
		Base:  capabilityBaseValidator{capabilities: Capabilities{MailRead: true, Send: true}},
		Extra: Capabilities{Calendar: true, Contacts: true, Labels: true, MailWrite: true},
	}
	capabilities, err := validator.Validate(context.Background(), ValidationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !capabilities.MailRead || !capabilities.Send || !capabilities.Calendar || !capabilities.Contacts || !capabilities.Labels || capabilities.MailWrite {
		t.Fatalf("capabilities = %#v", capabilities)
	}
}

func TestCapabilityValidatorFailsClosedWhenMailboxValidationFails(t *testing.T) {
	want := errors.New("mailbox unavailable")
	validator := CapabilityValidator{Base: capabilityBaseValidator{err: want}, Extra: Capabilities{Calendar: true}}
	capabilities, err := validator.Validate(context.Background(), ValidationRequest{})
	if !errors.Is(err, want) || capabilities.Calendar {
		t.Fatalf("Validate() = %#v, %v", capabilities, err)
	}

	if _, err := (CapabilityValidator{}).Validate(context.Background(), ValidationRequest{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("nil base error = %v", err)
	}
}
