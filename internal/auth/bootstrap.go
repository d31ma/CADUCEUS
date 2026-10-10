package auth

import (
	"context"
	"fmt"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// Operator credential provisioning.
//
// A newly provisioned mailbox holds no credential, and every credential
// operation in credentials.go requires an access token, which requires a
// credential. Something has to break that circle, and the honest place is the
// operator: these run from the command line, where authority is filesystem
// access to the deployment's key boundary, and they are deliberately absent
// from the bridge operation table so the legacy runtime can never reach them.
//
// The bootstrap pair is a password and one recovery code set. Recovery codes
// need no activation step, so they establish MFA assurance on first login,
// which is what lets the new owner enrol a passkey through the ordinary
// step-up path. TOTP is deliberately not used here: activating it would mean
// the operator proving possession of a secret that is supposed to be the
// mailbox owner's alone.

// SetInitialPassword stores the first password for one mailbox.
func (engine *Engine) SetInitialPassword(ctx context.Context, mailbox, password string) error {
	principal, err := engine.operatorPrincipal(ctx, mailbox)
	if err != nil {
		return err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if err := engine.client.SetPassword(callCtx, principal.ID, password); err != nil {
		return fmt.Errorf("set initial password for %s: %w", mailbox, err)
	}
	return nil
}

// IssueInitialRecoveryCodes returns one mailbox's recovery code set. The codes
// are returned once and replace any existing set.
func (engine *Engine) IssueInitialRecoveryCodes(
	ctx context.Context,
	mailbox string,
) (sesame.RecoveryCodeSet, error) {
	principal, err := engine.operatorPrincipal(ctx, mailbox)
	if err != nil {
		return sesame.RecoveryCodeSet{}, err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	codes, err := engine.client.RecoveryCodesIssue(callCtx, principal.ID)
	if err != nil {
		return sesame.RecoveryCodeSet{}, fmt.Errorf("issue recovery codes for %s: %w", mailbox, err)
	}
	return codes, nil
}

func (engine *Engine) operatorPrincipal(ctx context.Context, mailbox string) (sesame.Principal, error) {
	address, err := ParseAddress(mailbox)
	if err != nil {
		return sesame.Principal{}, err
	}
	tenantID, err := engine.TenantID(ctx)
	if err != nil {
		return sesame.Principal{}, err
	}
	return engine.principalByAddress(ctx, tenantID, address)
}

// TestBootstrap provisions one mailbox and returns its first credentials.
//
// It collapses four operator steps into one so a test suite can obtain a real
// token from the real engine rather than minting one of its own. Minting is
// what a test harness must never do here: a harness that can forge a token
// proves nothing about the boundary that verifies them.
//
// It is refused unless the runtime was started with test operations enabled,
// which production configuration rejects.
func (engine *Engine) TestBootstrap(
	ctx context.Context,
	mailbox, password, role string,
) (map[string]any, error) {
	if !engine.options.AllowTestOperations {
		return nil, Failure{
			Code:    CodeForbidden,
			Message: "test operations are not enabled on this runtime",
		}
	}
	address, err := ParseAddress(mailbox)
	if err != nil {
		return nil, err
	}
	if _, err := engine.ProvisionDomain(ctx, address.Domain); err != nil {
		return nil, err
	}
	provisioned, err := engine.ProvisionMailbox(ctx, address.String())
	if err != nil {
		return nil, err
	}
	if role == RoleDomainAdmin || role == RoleDomainViewer {
		if _, err := engine.GrantDomainRole(ctx, address.String(), address.Domain, role); err != nil {
			return nil, err
		}
	}
	if err := engine.SetInitialPassword(ctx, address.String(), password); err != nil {
		return nil, err
	}
	codes, err := engine.IssueInitialRecoveryCodes(ctx, address.String())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"address":       address.String(),
		"domain":        address.Domain,
		"principalId":   provisioned.PrincipalID,
		"tenantId":      provisioned.TenantID,
		"recoveryCodes": codes.Codes,
	}, nil
}
