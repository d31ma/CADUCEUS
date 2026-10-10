package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// ErrDenied reports an authenticated caller who may not perform the request.
var ErrDenied = errors.New("request is not permitted")

// Principal is the verified caller of one request, as every CADUCEUS feature
// sees it.
type Principal struct {
	PrincipalID string `json:"principalId"`
	TenantID    string `json:"tenantId"`
	SessionID   string `json:"sessionId"`
	Address     string `json:"address"`
	Domain      string `json:"domain"`
	Local       string `json:"local"`
	Scope       string `json:"scope"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// Decision is one authorization answer, carried back to the caller so a denial
// can be logged with the reason the engine gave.
type Decision struct {
	Principal     Principal `json:"principal"`
	Allowed       bool      `json:"allowed"`
	ReasonCode    string    `json:"reasonCode"`
	PolicyVersion int64     `json:"policyVersion"`
	DecisionID    string    `json:"decisionId"`
	// Domain is the mail domain the authorized resource belongs to. It is the
	// domain the caller may act in for this request, which is not necessarily
	// the domain of the caller's own mailbox: a member of another domain's
	// admin group acts in that domain. Storage ownership follows this value,
	// not the principal's address.
	Domain string `json:"domain,omitempty"`
}

// Authenticate proves one access token still speaks for a live session.
//
// The signature check is local and rejects a forged or expired token without
// touching the engine. The engine is then asked whether the grant still
// stands, because revocation is not something an already-signed token can
// report about itself. A token whose session was revoked one second ago fails
// here, not five minutes from now when it would have expired anyway.
//
// ponytail: two engine round trips per authorized request (this and the
// decision below). They are pipe operations on a local process and have not
// been measured as a bottleneck. If they ever are, the fix is a short-TTL
// positive cache keyed by (session, policy version) that any revocation event
// invalidates -- not skipping the check.
func (engine *Engine) Authenticate(ctx context.Context, token string) (Principal, error) {
	claims, err := engine.VerifyToken(ctx, token)
	if err != nil {
		return Principal{}, err
	}
	registration, found := engine.clientByID(claims.Audience)
	if !found {
		return Principal{}, fmt.Errorf("%w: audience is not a registered client", ErrToken)
	}
	callCtx, cancel := engine.call(ctx)
	introspection, err := engine.client.Introspect(
		callCtx, registration.ClientID, registration.Secret, token,
	)
	cancel()
	if err != nil {
		// An engine that cannot answer is not permission to proceed.
		return Principal{}, fmt.Errorf("introspect access token: %w", err)
	}
	if !introspection.Active {
		return Principal{}, fmt.Errorf("%w: the session behind it has ended", ErrToken)
	}
	if introspection.Subject != claims.Subject ||
		introspection.SessionID != claims.SessionID ||
		introspection.TenantID != claims.TenantID {
		return Principal{}, fmt.Errorf("%w: token claims disagree with the engine", ErrToken)
	}
	identity, err := engine.principalIdentity(ctx, claims.Subject)
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		PrincipalID: claims.Subject,
		TenantID:    claims.TenantID,
		SessionID:   claims.SessionID,
		Address:     identity.Address,
		Domain:      identity.Domain,
		Local:       identity.Local,
		Scope:       claims.Scope,
		ExpiresAt:   claims.ExpiresAt,
	}, nil
}

// Authorize authenticates one request and asks the engine whether it is
// permitted. It is the only way a CADUCEUS feature may act on a caller.
func (engine *Engine) Authorize(ctx context.Context, token, action, resource string) (Decision, error) {
	principal, err := engine.Authenticate(ctx, token)
	if err != nil {
		return Decision{}, err
	}
	decision, err := engine.Decide(ctx, principal, action, resource)
	if err != nil {
		return Decision{}, err
	}
	decision.Domain = resourceDomain(resource)
	return decision, nil
}

// resourceDomain reads the mail domain out of a resource pattern. Both shapes
// this boundary issues carry it in the second segment.
func resourceDomain(resource string) string {
	segments := strings.Split(resource, ":")
	if len(segments) < 2 {
		return ""
	}
	switch segments[0] {
	case "mailbox", "domain":
		return segments[1]
	default:
		return ""
	}
}

// Decide asks the engine one authorization question about an already
// authenticated principal.
func (engine *Engine) Decide(
	ctx context.Context,
	principal Principal,
	action, resource string,
) (Decision, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	answer, err := engine.client.Decide(callCtx, sesame.DecisionRequest{
		TenantID:    principal.TenantID,
		PrincipalID: principal.PrincipalID,
		Action:      action,
		Resource:    resource,
	}, nil)
	if err != nil {
		return Decision{}, fmt.Errorf("authorization decision: %w", err)
	}
	return Decision{
		Principal:     principal,
		Allowed:       answer.Decision == "allow",
		ReasonCode:    answer.ReasonCode,
		PolicyVersion: answer.PolicyVersion,
		DecisionID:    answer.DecisionID,
	}, nil
}

// AuthorizeMailbox is the common case: may this caller act on this mailbox.
// The mailbox is named by address, and an address in another domain is a
// resource this caller has no grant over, so it is denied by the same rule
// rather than by a special case.
func (engine *Engine) AuthorizeMailbox(
	ctx context.Context,
	token, action, mailbox string,
) (Decision, error) {
	address, err := ParseAddress(mailbox)
	if err != nil {
		return Decision{}, err
	}
	return engine.Authorize(ctx, token, action, MailboxResource(address))
}

// AuthorizeDomain is the domain-configuration case.
func (engine *Engine) AuthorizeDomain(
	ctx context.Context,
	token, action, domain string,
) (Decision, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return Decision{}, err
	}
	return engine.Authorize(ctx, token, action, DomainResource(normalized))
}
