package blackbox_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/auth"
)

// Phase 3 qualification. Every assertion here runs against the released SESAME
// engine and the released FYLO it stores its ledger in: the boundary is only
// worth as much as its behaviour against the real binaries.

const (
	testDomain   = "phase3.example"
	testMailbox  = "operator@phase3.example"
	testPassword = "correct horse battery staple"
)

func releasedSESAME(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("SESAME_TEST_BINARY")
	if binary == "" {
		t.Skip("SESAME_TEST_BINARY is required for released-engine qualification")
	}
	absolute, err := filepath.Abs(binary)
	requireNoError(t, err)
	return absolute
}

// startEngine initializes a deployment and starts the boundary against it.
func startEngine(t *testing.T, policy string) *auth.Engine {
	t.Helper()
	sesameBinary := releasedSESAME(t)
	fyloBinary := releasedFYLO(t)
	deployment := filepath.Join(t.TempDir(), "deployment")

	initContext, cancelInit := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelInit()
	initialize := exec.CommandContext(
		initContext, sesameBinary, "init",
		"--deployment", deployment,
		"--fylo-binary", fyloBinary,
		"--issuer", "https://mail.phase3.example",
	)
	if output, err := initialize.CombinedOutput(); err != nil {
		t.Fatalf("sesame init failed: %v\n%s", err, output)
	}

	engine, err := auth.Start(context.Background(), auth.Options{
		Binary:          sesameBinary,
		Deployment:      deployment,
		ExpectedVersion: "26.33.02",
		Issuer:          "https://mail.phase3.example",
		RedirectURI:     "https://mail.phase3.example/auth/callback",
		Policy:          policy,
		SessionLifetime: time.Hour,
		RequestTimeout:  15 * time.Second,
		Stderr:          nil,
	})
	requireNoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

// provision creates the domain and one mailbox with a password credential.
func provision(t *testing.T, engine *auth.Engine) auth.Provisioning {
	t.Helper()
	ctx := context.Background()
	_, err := engine.ProvisionDomain(ctx, testDomain)
	requireNoError(t, err)
	provisioned, err := engine.ProvisionMailbox(ctx, testMailbox)
	requireNoError(t, err)
	if !provisioned.Created || provisioned.PrincipalID == "" {
		t.Fatalf("mailbox was not provisioned: %+v", provisioned)
	}
	// Repeating provisioning must not fail: an operator re-running the
	// bootstrap is the normal case, not an error.
	repeated, err := engine.ProvisionMailbox(ctx, testMailbox)
	requireNoError(t, err)
	if repeated.Created {
		t.Fatal("repeated provisioning reported a new mailbox")
	}
	return provisioned
}

// bootstrapCredentials gives one mailbox its first password and recovery code
// set, exactly as `caduceus auth credentials` does for an operator.
func bootstrapCredentials(t *testing.T, engine *auth.Engine) []string {
	t.Helper()
	ctx := context.Background()
	requireNoError(t, engine.SetInitialPassword(ctx, testMailbox, testPassword))
	codes, err := engine.IssueInitialRecoveryCodes(ctx, testMailbox)
	requireNoError(t, err)
	if len(codes.Codes) < 4 {
		t.Fatalf("recovery code set is too small: %d", len(codes.Codes))
	}
	return codes.Codes
}

// login drives a complete authentication under the passkey-first policy using
// a recovery code as the second factor. A WebAuthn assertion needs a browser
// authenticator; a recovery code reaches the same MFA assurance and is what a
// newly provisioned mailbox actually holds.
func login(t *testing.T, engine *auth.Engine, codes []string) auth.TokenSet {
	t.Helper()
	ctx := context.Background()

	begun, err := engine.LoginBegin(ctx, testMailbox)
	requireNoError(t, err)
	if begun.TransactionID == "" {
		t.Fatal("login begin issued no transaction")
	}
	if begun.Satisfied {
		t.Fatal("a transaction with no verified factor reported as satisfied")
	}

	afterPassword, err := engine.VerifyPassword(ctx, begun.TransactionID, testPassword)
	requireNoError(t, err)
	if afterPassword.Satisfied {
		t.Fatal("a password alone satisfied the policy")
	}

	afterRecovery, err := engine.VerifyRecoveryCode(ctx, begun.TransactionID, codes[0])
	requireNoError(t, err)
	if !afterRecovery.Satisfied {
		t.Fatalf("password and recovery code did not satisfy the policy: %+v", afterRecovery)
	}

	tokens, err := engine.LoginComplete(ctx, begun.TransactionID)
	requireNoError(t, err)
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.IDToken == "" {
		t.Fatalf("incomplete token set: %+v", tokens)
	}
	if tokens.Address != testMailbox || tokens.Domain != testDomain {
		t.Fatalf("token set names the wrong mailbox: %+v", tokens)
	}
	return tokens
}

func TestPhase3LoginIssuesVerifiableTokens(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))

	claims, err := engine.VerifyToken(context.Background(), tokens.AccessToken)
	requireNoError(t, err)
	if claims.Issuer != "https://mail.phase3.example" {
		t.Fatalf("issuer = %q", claims.Issuer)
	}
	if claims.SessionID != tokens.SessionID || claims.Subject != tokens.PrincipalID {
		t.Fatalf("claims disagree with the issued session: %+v", claims)
	}

	principal, err := engine.Authenticate(context.Background(), tokens.AccessToken)
	requireNoError(t, err)
	if principal.Address != testMailbox {
		t.Fatalf("authenticated address = %q", principal.Address)
	}
}

func TestPhase3AuthorizationIsDefaultDeny(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))
	ctx := context.Background()

	own, err := engine.AuthorizeMailbox(ctx, tokens.AccessToken, auth.ActionMailboxRead, testMailbox)
	requireNoError(t, err)
	if !own.Allowed {
		t.Fatalf("a mailbox owner was denied its own mailbox: %+v", own)
	}

	// Another mailbox in the same domain is a different resource, and the
	// owner role covers exactly one.
	sibling, err := engine.AuthorizeMailbox(
		ctx, tokens.AccessToken, auth.ActionMailboxRead, "someone-else@"+testDomain,
	)
	requireNoError(t, err)
	if sibling.Allowed {
		t.Fatal("one mailbox owner reached another mailbox in the same domain")
	}

	crossDomain, err := engine.AuthorizeMailbox(
		ctx, tokens.AccessToken, auth.ActionMailboxRead, "operator@other.example",
	)
	requireNoError(t, err)
	if crossDomain.Allowed {
		t.Fatal("a mailbox owner reached a mailbox in another domain")
	}

	domainAdmin, err := engine.AuthorizeDomain(
		ctx, tokens.AccessToken, auth.ActionDomainAdmin, testDomain,
	)
	requireNoError(t, err)
	if domainAdmin.Allowed {
		t.Fatal("a plain mailbox owner holds domain administration")
	}
}

func TestPhase3DomainRoleGrantsWholeDomain(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	ctx := context.Background()

	if _, err := engine.GrantDomainRole(ctx, testMailbox, testDomain, auth.RoleDomainAdmin); err != nil {
		t.Fatalf("grant domain admin: %v", err)
	}
	tokens := login(t, engine, codes)

	for _, check := range []struct {
		action   string
		mailbox  string
		domain   string
		expected bool
	}{
		{action: auth.ActionMailboxRead, mailbox: "anyone@" + testDomain, expected: true},
		{action: auth.ActionMailboxSend, mailbox: "anyone@" + testDomain, expected: true},
		{action: auth.ActionMailboxRead, mailbox: "anyone@other.example", expected: false},
		{action: auth.ActionDomainAdmin, domain: testDomain, expected: true},
		{action: auth.ActionDomainAdmin, domain: "other.example", expected: false},
	} {
		var decision auth.Decision
		var err error
		if check.mailbox != "" {
			decision, err = engine.AuthorizeMailbox(ctx, tokens.AccessToken, check.action, check.mailbox)
		} else {
			decision, err = engine.AuthorizeDomain(ctx, tokens.AccessToken, check.action, check.domain)
		}
		requireNoError(t, err)
		if decision.Allowed != check.expected {
			t.Fatalf(
				"%s on %s%s allowed = %v, want %v (%s)",
				check.action, check.mailbox, check.domain,
				decision.Allowed, check.expected, decision.ReasonCode,
			)
		}
	}
}

func TestPhase3RevokedSessionFailsClosedImmediately(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))
	ctx := context.Background()

	// The token is signed, unexpired, and still verifies locally.
	requireNoError(t, engine.RevokeSession(ctx, tokens.SessionID, "qualification"))
	if _, err := engine.VerifyToken(ctx, tokens.AccessToken); err != nil {
		t.Fatalf("a revoked session must not invalidate the signature: %v", err)
	}

	// Acting on it must fail anyway, and immediately: not when it expires.
	if _, err := engine.Authenticate(ctx, tokens.AccessToken); err == nil {
		t.Fatal("a revoked session still authenticated")
	}
	if _, err := engine.AuthorizeMailbox(
		ctx, tokens.AccessToken, auth.ActionMailboxRead, testMailbox,
	); err == nil {
		t.Fatal("a revoked session still reached an authorization decision")
	}
	// The refresh family rests on the session, so it ends with it.
	if _, err := engine.Refresh(ctx, tokens.RefreshToken, clientID(t, tokens.AccessToken)); err == nil {
		t.Fatal("a revoked session still refreshed")
	}
}

func TestPhase3SuspendedPrincipalFailsClosed(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))
	ctx := context.Background()

	requireNoError(t, engine.SuspendMailbox(ctx, testMailbox))
	if _, err := engine.Authenticate(ctx, tokens.AccessToken); err == nil {
		t.Fatal("a suspended mailbox still authenticated")
	}
}

func TestPhase3RefreshRotatesAndDetectsReuse(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))
	ctx := context.Background()
	client := clientID(t, tokens.AccessToken)

	rotated, err := engine.Refresh(ctx, tokens.RefreshToken, client)
	requireNoError(t, err)
	if rotated.RefreshToken == tokens.RefreshToken {
		t.Fatal("the refresh token did not rotate")
	}
	if rotated.SessionID != tokens.SessionID {
		t.Fatal("refresh moved the session")
	}

	// Replaying the consumed token ends the whole family: that is the point of
	// rotation, and the replacement must stop working too.
	if _, err := engine.Refresh(ctx, tokens.RefreshToken, client); err == nil {
		t.Fatal("a consumed refresh token was accepted")
	}
	if _, err := engine.Refresh(ctx, rotated.RefreshToken, client); err == nil {
		t.Fatal("refresh reuse did not end the family")
	}
}

func TestPhase3MalformedAndForgedTokensFailClosed(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	tokens := login(t, engine, bootstrapCredentials(t, engine))
	ctx := context.Background()

	parts := strings.Split(tokens.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("issued token is not a JWS: %q", tokens.AccessToken)
	}
	tampered := tamperClaims(t, parts)

	for name, token := range map[string]string{
		"empty":              "",
		"not a token":        "definitely-not-a-token",
		"two segments":       parts[0] + "." + parts[1],
		"empty signature":    parts[0] + "." + parts[1] + ".",
		"truncated payload":  parts[0] + "." + parts[1][:len(parts[1])-4] + "." + parts[2],
		"swapped signature":  parts[0] + "." + parts[1] + "." + reverse(parts[2]),
		"algorithm none":     noneAlgorithm(t, parts[1]),
		"tampered tenant":    tampered,
		"id token as access": tokens.IDToken,
	} {
		if _, err := engine.Authenticate(ctx, token); err == nil {
			t.Fatalf("%s was accepted as an access token", name)
		}
	}
}

func TestPhase3PolicyRequiresPasswordWhenConfigured(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasswordAndPasskey)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	ctx := context.Background()

	begun, err := engine.LoginBegin(ctx, testMailbox)
	requireNoError(t, err)
	if got := strings.Join(begun.RequiredFactors, ","); got != "password,passkey" {
		t.Fatalf("required factors = %q", got)
	}

	// A recovery code is a second factor: the engine closes a transaction that
	// is offered one before any primary factor.
	if _, err := engine.VerifyRecoveryCode(ctx, begun.TransactionID, codes[0]); err == nil {
		t.Fatal("a recovery code was accepted as the first factor")
	}

	// The full ladder under this policy: password, then a second factor.
	second, err := engine.LoginBegin(ctx, testMailbox)
	requireNoError(t, err)
	afterPassword, err := engine.VerifyPassword(ctx, second.TransactionID, testPassword)
	requireNoError(t, err)
	if afterPassword.Satisfied {
		t.Fatal("a password alone satisfied the password-and-passkey policy")
	}
	if _, err := engine.LoginComplete(ctx, second.TransactionID); err == nil {
		t.Fatal("login completed at password assurance")
	}
	afterRecovery, err := engine.VerifyRecoveryCode(ctx, second.TransactionID, codes[1])
	requireNoError(t, err)
	if afterRecovery.Assurance != auth.AssuranceMFA {
		t.Fatalf("a recovery code did not reach MFA assurance: %+v", afterRecovery)
	}
	if !afterRecovery.Satisfied {
		t.Fatalf("password and a second factor did not satisfy the policy: %+v", afterRecovery)
	}
	// The earlier rejected completion must not have been terminal: a caller
	// that supplies the missing factor can still finish the same login.
	tokens, err := engine.LoginComplete(ctx, second.TransactionID)
	requireNoError(t, err)
	if tokens.AccessToken == "" {
		t.Fatal("the recovered login issued no token")
	}
}

func TestPhase3CredentialChangesRequireStepUp(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	tokens := login(t, engine, codes)
	ctx := context.Background()

	if _, err := engine.RecoveryCodesIssue(ctx, tokens.AccessToken, "atx_not_a_transaction"); err == nil {
		t.Fatal("recovery codes were issued without a step-up")
	}

	stepUp, err := engine.LoginBegin(ctx, testMailbox)
	requireNoError(t, err)
	if _, err := engine.VerifyPassword(ctx, stepUp.TransactionID, testPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.VerifyRecoveryCode(ctx, stepUp.TransactionID, codes[1]); err != nil {
		t.Fatal(err)
	}
	reissued, err := engine.RecoveryCodesIssue(ctx, tokens.AccessToken, stepUp.TransactionID)
	requireNoError(t, err)
	if len(reissued.Codes) == 0 {
		t.Fatal("no recovery codes were issued")
	}

	// A step-up is single use: the same transaction cannot authorize a second
	// credential change.
	if _, err := engine.RecoveryCodesIssue(ctx, tokens.AccessToken, stepUp.TransactionID); err == nil {
		t.Fatal("a consumed step-up authorized a second credential change")
	}
}

func TestPhase3BridgeOperationsRejectUnknownAndMalformed(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	ctx := context.Background()

	if _, err := engine.Dispatch(ctx, "auth.not.a.real.operation", json.RawMessage(`{}`)); err == nil {
		t.Fatal("an unknown operation was dispatched")
	}
	// A raw engine operation must not be reachable through the boundary.
	if _, err := engine.Dispatch(ctx, "authn.begin", json.RawMessage(`{}`)); err == nil {
		t.Fatal("a raw engine operation was reachable through the boundary")
	}
	// Unknown fields are refused rather than ignored, so a client that thinks
	// it is passing a parameter learns that it is not.
	if _, err := engine.Dispatch(
		ctx, "auth.login.begin", json.RawMessage(`{"address":"a@b.example","admin":true}`),
	); err == nil {
		t.Fatal("an unknown request field was ignored")
	}
	if _, err := engine.Dispatch(ctx, "auth.login.begin", json.RawMessage(`{}`)); err == nil {
		t.Fatal("a missing required field was accepted")
	}
}

func TestPhase3OperationsAreAllPrefixed(t *testing.T) {
	operations := auth.Operations()
	if len(operations) == 0 {
		t.Fatal("the boundary publishes no operations")
	}
	for _, operation := range operations {
		if !auth.Handles(operation) {
			t.Fatalf("operation %q is outside the auth namespace", operation)
		}
	}
}

// clientID reads the audience the engine minted a token for. Only the boundary
// knows the registered client, and a refresh has to name it.
func clientID(t *testing.T, accessToken string) string {
	t.Helper()
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a JWS: %q", accessToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	requireNoError(t, err)
	var claims struct {
		Audience string `json:"aud"`
	}
	requireNoError(t, json.Unmarshal(payload, &claims))
	return claims.Audience
}

// tamperClaims re-encodes the payload with a different tenant, keeping the
// original signature. It is the "edit the claims and hope nobody checks" case.
func tamperClaims(t *testing.T, parts []string) string {
	t.Helper()
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	requireNoError(t, err)
	var claims map[string]any
	requireNoError(t, json.Unmarshal(payload, &claims))
	claims["tenant_id"] = "tnt_00000000000000000000000000000000"
	edited, err := json.Marshal(claims)
	requireNoError(t, err)
	return parts[0] + "." + base64.RawURLEncoding.EncodeToString(edited) + "." + parts[2]
}

// noneAlgorithm builds the unsigned-token downgrade.
func noneAlgorithm(t *testing.T, payload string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	return header + "." + payload + "."
}

func reverse(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// The reason CADUCEUS runs one tenant rather than one per mail domain: SESAME
// group membership requires the principal and the group to share a tenant, so a
// domain-per-tenant model would make one person administering two domains two
// identities with two logins. These prove the alternative actually works.

const secondDomain = "phase3-second.example"

func TestPhase3OneIdentityAdministersSeveralDomains(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	ctx := context.Background()

	// A second mail domain, with no mailbox of its own for this principal.
	if _, err := engine.ProvisionDomain(ctx, secondDomain); err != nil {
		t.Fatalf("provision second domain: %v", err)
	}
	third := "phase3-third.example"
	if _, err := engine.ProvisionDomain(ctx, third); err != nil {
		t.Fatalf("provision third domain: %v", err)
	}

	// One mailbox joins two domains' admin groups and one domain's viewer group.
	for _, membership := range []struct {
		domain string
		role   string
	}{
		{domain: testDomain, role: auth.RoleDomainAdmin},
		{domain: secondDomain, role: auth.RoleDomainAdmin},
		{domain: third, role: auth.RoleDomainViewer},
	} {
		joined, err := engine.GrantDomainRole(ctx, testMailbox, membership.domain, membership.role)
		requireNoError(t, err)
		if !joined.Added || joined.Domain != membership.domain {
			t.Fatalf("membership not recorded: %+v", joined)
		}
	}

	// One login. One token. Authority in three domains.
	tokens := login(t, engine, codes)
	for _, check := range []struct {
		action   string
		mailbox  string
		domain   string
		expected bool
		why      string
	}{
		{action: auth.ActionDomainAdmin, domain: testDomain, expected: true,
			why: "admin of its own domain"},
		{action: auth.ActionDomainAdmin, domain: secondDomain, expected: true,
			why: "admin of a domain it has no mailbox in"},
		{action: auth.ActionMailboxWrite, mailbox: "anyone@" + secondDomain, expected: true,
			why: "writes a mailbox in the second domain"},
		{action: auth.ActionMailboxRead, mailbox: "anyone@" + third, expected: true,
			why: "reads a mailbox in the viewer domain"},
		{action: auth.ActionMailboxWrite, mailbox: "anyone@" + third, expected: false,
			why: "viewer authority does not write"},
		{action: auth.ActionDomainAdmin, domain: third, expected: false,
			why: "viewer authority does not administer"},
		{action: auth.ActionDomainAdmin, domain: "phase3-unrelated.example", expected: false,
			why: "no membership, no authority"},
	} {
		decision := decide(t, engine, tokens.AccessToken, check.action, check.mailbox, check.domain)
		if decision.Allowed != check.expected {
			t.Fatalf("%s: allowed = %v, want %v (%s)",
				check.why, decision.Allowed, check.expected, decision.ReasonCode)
		}
	}
}

func TestPhase3DomainAuthorityIsRevokedPerDomain(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	ctx := context.Background()

	if _, err := engine.ProvisionDomain(ctx, secondDomain); err != nil {
		t.Fatalf("provision second domain: %v", err)
	}
	for _, domain := range []string{testDomain, secondDomain} {
		if _, err := engine.GrantDomainRole(ctx, testMailbox, domain, auth.RoleDomainAdmin); err != nil {
			t.Fatalf("grant admin on %s: %v", domain, err)
		}
	}

	// Leaving one domain's group takes that domain's authority and no other's.
	if _, err := engine.RevokeDomainRole(ctx, testMailbox, secondDomain, auth.RoleDomainAdmin); err != nil {
		t.Fatalf("revoke second domain: %v", err)
	}

	tokens := login(t, engine, codes)
	kept := decide(t, engine, tokens.AccessToken, auth.ActionDomainAdmin, "", testDomain)
	if !kept.Allowed {
		t.Fatalf("revoking one domain removed authority over another: %s", kept.ReasonCode)
	}
	lost := decide(t, engine, tokens.AccessToken, auth.ActionDomainAdmin, "", secondDomain)
	if lost.Allowed {
		t.Fatal("authority survived leaving the domain group")
	}
}

func TestPhase3DecisionNamesTheDomainItAuthorized(t *testing.T) {
	engine := startEngine(t, auth.PolicyPasskeyFirst)
	provision(t, engine)
	codes := bootstrapCredentials(t, engine)
	ctx := context.Background()

	if _, err := engine.ProvisionDomain(ctx, secondDomain); err != nil {
		t.Fatalf("provision second domain: %v", err)
	}
	if _, err := engine.GrantDomainRole(ctx, testMailbox, secondDomain, auth.RoleDomainAdmin); err != nil {
		t.Fatalf("grant admin on the second domain: %v", err)
	}
	tokens := login(t, engine, codes)

	// Storage ownership follows the domain the decision authorized, not the
	// caller's own address, or a cross-domain admin would write into its home
	// domain's group.
	decision, err := engine.AuthorizeMailbox(
		ctx, tokens.AccessToken, auth.ActionMailboxWrite, "anyone@"+secondDomain,
	)
	requireNoError(t, err)
	if !decision.Allowed {
		t.Fatalf("cross-domain write denied: %s", decision.ReasonCode)
	}
	if decision.Domain != secondDomain {
		t.Fatalf("decision domain = %q, want %q", decision.Domain, secondDomain)
	}
	if decision.Principal.Domain != testDomain {
		t.Fatalf("principal domain = %q, want its own %q", decision.Principal.Domain, testDomain)
	}
}

// decide runs one authorization question against a mailbox or a domain.
func decide(
	t *testing.T,
	engine *auth.Engine,
	token, action, mailbox, domain string,
) auth.Decision {
	t.Helper()
	var decision auth.Decision
	var err error
	if mailbox != "" {
		decision, err = engine.AuthorizeMailbox(context.Background(), token, action, mailbox)
	} else {
		decision, err = engine.AuthorizeDomain(context.Background(), token, action, domain)
	}
	requireNoError(t, err)
	return decision
}
