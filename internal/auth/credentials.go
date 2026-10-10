package auth

import (
	"context"
	"fmt"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// Changing a credential is the action an account takeover needs, so it is not
// permitted on the strength of a session that was established at some point in
// the past. The caller must satisfy a fresh authentication transaction, and
// that transaction must belong to the same principal as the access token.
//
// Proving "same principal" without an enumeration oracle is why this completes
// the transaction rather than inspecting it: the engine deliberately will not
// say whose transaction it is, but it will issue a session, and that session
// names its principal. The session is proof only, so it is revoked as soon as
// it has been compared.
func (engine *Engine) withStepUp(
	ctx context.Context,
	token, stepUpTransactionID string,
	action func(principal Principal) error,
) error {
	principal, err := engine.Authenticate(ctx, token)
	if err != nil {
		return err
	}
	state, err := engine.transactionState(ctx, stepUpTransactionID)
	if err != nil {
		return err
	}
	if !state.Satisfied {
		return fmt.Errorf("%w: %s", ErrPolicy, missingFactorReason(engine.options.Policy, state))
	}
	completeCtx, cancel := engine.call(ctx)
	session, err := engine.client.AuthenticationComplete(
		completeCtx, stepUpTransactionID, engine.options.SessionLifetime,
	)
	cancel()
	engine.ledger.forget(stepUpTransactionID)
	if err != nil {
		return fmt.Errorf("complete step-up: %w", err)
	}
	// The proof session ends whether or not the comparison succeeds.
	defer func() {
		_ = engine.RevokeSession(context.WithoutCancel(ctx), session.SessionID, "step-up proof consumed")
	}()

	if session.PrincipalID != principal.PrincipalID || session.TenantID != principal.TenantID {
		return fmt.Errorf("%w: the step-up proves a different principal", ErrDenied)
	}
	if session.Assurance != AssuranceMFA {
		return fmt.Errorf("%w: step-up assurance is %s", ErrPolicy, session.Assurance)
	}
	return action(principal)
}

// PasskeyRegisterBegin issues a registration challenge for the caller's own
// account.
func (engine *Engine) PasskeyRegisterBegin(
	ctx context.Context,
	token, stepUpTransactionID string,
) (sesame.PasskeyRegistrationRequest, error) {
	var request sesame.PasskeyRegistrationRequest
	err := engine.withStepUp(ctx, token, stepUpTransactionID, func(principal Principal) error {
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		issued, err := engine.client.PasskeyRegisterBegin(callCtx, principal.PrincipalID)
		if err != nil {
			return fmt.Errorf("begin passkey registration: %w", err)
		}
		request = issued
		return nil
	})
	return request, err
}

// PasskeyRegisterFinish stores a verified credential for the caller.
//
// It is deliberately not behind a second step-up: the challenge this answers
// was issued to a stepped-up caller and is single-use and short-lived, and the
// principal is taken from the token rather than from the request body.
func (engine *Engine) PasskeyRegisterFinish(
	ctx context.Context,
	token string,
	attestationObject, clientDataJSON []byte,
) (sesame.Passkey, error) {
	principal, err := engine.Authenticate(ctx, token)
	if err != nil {
		return sesame.Passkey{}, err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	passkey, err := engine.client.PasskeyRegisterFinish(
		callCtx, principal.PrincipalID, attestationObject, clientDataJSON,
	)
	if err != nil {
		return sesame.Passkey{}, fmt.Errorf("finish passkey registration: %w", err)
	}
	return passkey, nil
}

// PasskeyList returns the caller's own registered credentials.
func (engine *Engine) PasskeyList(ctx context.Context, token string) ([]sesame.Passkey, error) {
	principal, err := engine.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	passkeys, err := engine.client.PasskeyList(callCtx, principal.PrincipalID)
	if err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}
	return passkeys, nil
}

// PasskeyRemove unregisters one of the caller's own credentials.
func (engine *Engine) PasskeyRemove(ctx context.Context, token, stepUpTransactionID, credentialID string) error {
	return engine.withStepUp(ctx, token, stepUpTransactionID, func(principal Principal) error {
		owned, err := engine.ownsCredential(ctx, principal.PrincipalID, credentialID)
		if err != nil {
			return err
		}
		if !owned {
			// Removing someone else's credential is a denial, and it reports
			// the same way as removing one that does not exist.
			return fmt.Errorf("%w: credential does not belong to this principal", ErrDenied)
		}
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		if err := engine.client.PasskeyRemove(callCtx, credentialID); err != nil {
			return fmt.Errorf("remove passkey: %w", err)
		}
		return nil
	})
}

func (engine *Engine) ownsCredential(ctx context.Context, principalID, credentialID string) (bool, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	passkeys, err := engine.client.PasskeyList(callCtx, principalID)
	if err != nil {
		return false, fmt.Errorf("list passkeys: %w", err)
	}
	for _, passkey := range passkeys {
		if passkey.CredentialID == credentialID {
			return true, nil
		}
	}
	return false, nil
}

// TOTPEnroll issues a shared secret for the caller. The secret is returned
// once and the authenticator is unusable until TOTPActivate proves a code.
func (engine *Engine) TOTPEnroll(
	ctx context.Context,
	token, stepUpTransactionID string,
) (sesame.TOTPEnrollment, error) {
	var enrollment sesame.TOTPEnrollment
	err := engine.withStepUp(ctx, token, stepUpTransactionID, func(principal Principal) error {
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		issued, err := engine.client.TOTPEnroll(callCtx, principal.PrincipalID, engine.options.Issuer)
		if err != nil {
			return fmt.Errorf("enroll TOTP: %w", err)
		}
		enrollment = issued
		return nil
	})
	return enrollment, err
}

// TOTPActivate proves one code and makes the enrolled authenticator usable.
func (engine *Engine) TOTPActivate(ctx context.Context, token, code string) error {
	principal, err := engine.Authenticate(ctx, token)
	if err != nil {
		return err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if err := engine.client.TOTPActivate(callCtx, principal.PrincipalID, code); err != nil {
		return fmt.Errorf("activate TOTP: %w", err)
	}
	return nil
}

// RecoveryCodesIssue replaces the caller's recovery code set. The codes are
// returned once.
func (engine *Engine) RecoveryCodesIssue(
	ctx context.Context,
	token, stepUpTransactionID string,
) (sesame.RecoveryCodeSet, error) {
	var codes sesame.RecoveryCodeSet
	err := engine.withStepUp(ctx, token, stepUpTransactionID, func(principal Principal) error {
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		issued, err := engine.client.RecoveryCodesIssue(callCtx, principal.PrincipalID)
		if err != nil {
			return fmt.Errorf("issue recovery codes: %w", err)
		}
		codes = issued
		return nil
	})
	return codes, err
}

// SetPassword stores a password verifier for the caller.
func (engine *Engine) SetPassword(ctx context.Context, token, stepUpTransactionID, password string) error {
	return engine.withStepUp(ctx, token, stepUpTransactionID, func(principal Principal) error {
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		if err := engine.client.SetPassword(callCtx, principal.PrincipalID, password); err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		return nil
	})
}
