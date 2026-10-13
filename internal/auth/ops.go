package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// OperationPrefix marks every operation this boundary answers. The legacy
// runtime reaches authentication only through these composed operations: raw
// engine operations are deliberately not proxied, because orchestrating them
// is the authorization boundary itself and it lives here.
const OperationPrefix = "auth."

// Failure is a stable, client-safe error from one auth operation.
type Failure struct {
	Code    string
	Message string
}

func (failure Failure) Error() string {
	return failure.Code + ": " + failure.Message
}

// Stable operation error codes. They are the vocabulary the legacy routes map
// onto HTTP status codes, and they never distinguish an unknown identifier
// from a wrong credential.
const (
	CodeBadRequest   = "EBADREQUEST"
	CodeUnauthorized = "EUNAUTHORIZED"
	CodeForbidden    = "EFORBIDDEN"
	CodePolicy       = "EPOLICY"
	CodeConflict     = "ECONFLICT"
	CodeUnavailable  = "EUNAVAILABLE"
)

// Handles reports whether one operation name belongs to this boundary.
func Handles(operation string) bool {
	return strings.HasPrefix(operation, OperationPrefix)
}

// Operations lists every operation this boundary answers, sorted. The bridge
// uses it as its allowlist so an unknown name is refused before it reaches a
// handler.
func Operations() []string {
	names := make([]string, 0, len(handlers))
	for name := range handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type handler func(context.Context, *Engine, json.RawMessage) (any, error)

// Dispatch answers one composed operation. The request is the decoded bridge
// payload; the result is encoded by the caller.
func (engine *Engine) Dispatch(ctx context.Context, operation string, payload json.RawMessage) (any, error) {
	route, found := handlers[operation]
	if !found {
		return nil, Failure{Code: CodeBadRequest, Message: "unknown auth operation " + operation}
	}
	result, err := route(ctx, engine, payload)
	if err != nil {
		return nil, classify(err)
	}
	return result, nil
}

// classify maps an internal error onto the stable client-facing vocabulary.
// Anything unrecognized becomes "unavailable" rather than leaking its text.
func classify(err error) error {
	var failure Failure
	if errors.As(err, &failure) {
		return failure
	}
	switch {
	case errors.Is(err, ErrToken):
		return Failure{Code: CodeUnauthorized, Message: "authentication required"}
	case errors.Is(err, ErrDenied):
		return Failure{Code: CodeForbidden, Message: "request is not permitted"}
	case errors.Is(err, ErrPolicy):
		return Failure{Code: CodePolicy, Message: err.Error()}
	case hasCode(err, codeIdentifierConflict), hasCode(err, codeRoleExists),
		hasCode(err, codeGrantExists), hasCode(err, codeGroupExists),
		hasCode(err, codeGroupMemberExists):
		return Failure{Code: CodeConflict, Message: "already provisioned"}
	case hasCode(err, codePrincipalNotFound), hasCode(err, codeTenantNotFound):
		// Reporting "no such mailbox" from an authenticated administrative
		// operation is fine; reporting it from a login is not, and login never
		// reaches here.
		return Failure{Code: CodeBadRequest, Message: "mailbox or domain is not provisioned"}
	default:
		return Failure{Code: CodeUnavailable, Message: err.Error()}
	}
}

func decode[T any](payload json.RawMessage) (T, error) {
	var request T
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, Failure{Code: CodeBadRequest, Message: "invalid request: " + err.Error()}
	}
	if decoder.More() {
		return request, Failure{Code: CodeBadRequest, Message: "request must contain one JSON object"}
	}
	return request, nil
}

// decodeDomainRole reads one membership request. The domain is explicit and
// separate from the address on purpose: the whole point of a domain group is
// that a mailbox may administer a domain that is not its own.
func decodeDomainRole(payload json.RawMessage) (domainRoleRequest, error) {
	request, err := decode[domainRoleRequest](payload)
	if err != nil {
		return domainRoleRequest{}, err
	}
	if err := required("address", request.Address); err != nil {
		return domainRoleRequest{}, err
	}
	if err := required("domain", request.Domain); err != nil {
		return domainRoleRequest{}, err
	}
	if err := required("role", request.Role); err != nil {
		return domainRoleRequest{}, err
	}
	return request, nil
}

func decodeBase64(name, value string) ([]byte, error) {
	if value == "" {
		return nil, Failure{Code: CodeBadRequest, Message: name + " is required"}
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, Failure{Code: CodeBadRequest, Message: name + " is not base64"}
	}
	return decoded, nil
}

func required(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return Failure{Code: CodeBadRequest, Message: name + " is required"}
	}
	return nil
}

// Request shapes. Each is decoded strictly: an unknown field is a bad request
// rather than a silently ignored one.

type addressRequest struct {
	Address string `json:"address"`
}

type transactionRequest struct {
	TransactionID string `json:"transactionId"`
}

type passwordRequest struct {
	TransactionID string `json:"transactionId"`
	Password      string `json:"password"`
}

type codeRequest struct {
	TransactionID string `json:"transactionId"`
	Code          string `json:"code"`
}

type passkeyAssertionRequest struct {
	TransactionID     string `json:"transactionId"`
	CredentialID      string `json:"credentialId"`
	AuthenticatorData string `json:"authenticatorData"`
	ClientDataJSON    string `json:"clientDataJSON"`
	Signature         string `json:"signature"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
	ClientID     string `json:"clientId"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

type authorizeRequest struct {
	Token    string `json:"token"`
	Action   string `json:"action"`
	Mailbox  string `json:"mailbox,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Resource string `json:"resource,omitempty"`
}

type stepUpRequest struct {
	Token               string `json:"token"`
	StepUpTransactionID string `json:"stepUpTransactionId"`
}

type passkeyRegisterRequest struct {
	Token             string `json:"token"`
	AttestationObject string `json:"attestationObject"`
	ClientDataJSON    string `json:"clientDataJSON"`
}

type passkeyRemoveRequest struct {
	Token               string `json:"token"`
	StepUpTransactionID string `json:"stepUpTransactionId"`
	CredentialID        string `json:"credentialId"`
}

type setPasswordRequest struct {
	Token               string `json:"token"`
	StepUpTransactionID string `json:"stepUpTransactionId"`
	Password            string `json:"password"`
}

type activateRequest struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

type logoutRequest struct {
	IDToken string `json:"idToken"`
}

type revokeRequest struct {
	Token  string `json:"token"`
	Reason string `json:"reason,omitempty"`
}

type provisionDomainRequest struct {
	Domain string `json:"domain"`
}

type provisionMailboxRequest struct {
	Address string `json:"address"`
}

type domainRoleRequest struct {
	Address string `json:"address"`
	Domain  string `json:"domain"`
	Role    string `json:"role"`
}

type testBootstrapRequest struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

var handlers = map[string]handler{
	"auth.login.begin": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[addressRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("address", request.Address); err != nil {
			return nil, err
		}
		return engine.LoginBegin(ctx, request.Address)
	},

	"auth.login.password": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[passwordRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		if err := required("password", request.Password); err != nil {
			return nil, err
		}
		return engine.VerifyPassword(ctx, request.TransactionID, request.Password)
	},

	"auth.login.passkey.options": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[transactionRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		return engine.PasskeyOptions(ctx, request.TransactionID)
	},

	"auth.login.passkey": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[passkeyAssertionRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		if err := required("credentialId", request.CredentialID); err != nil {
			return nil, err
		}
		authenticatorData, err := decodeBase64("authenticatorData", request.AuthenticatorData)
		if err != nil {
			return nil, err
		}
		clientData, err := decodeBase64("clientDataJSON", request.ClientDataJSON)
		if err != nil {
			return nil, err
		}
		signature, err := decodeBase64("signature", request.Signature)
		if err != nil {
			return nil, err
		}
		return engine.VerifyPasskey(
			ctx, request.TransactionID, request.CredentialID,
			authenticatorData, clientData, signature,
		)
	},

	"auth.login.totp": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[codeRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		if err := required("code", request.Code); err != nil {
			return nil, err
		}
		return engine.VerifyTOTP(ctx, request.TransactionID, request.Code)
	},

	"auth.login.recovery": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[codeRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		if err := required("code", request.Code); err != nil {
			return nil, err
		}
		return engine.VerifyRecoveryCode(ctx, request.TransactionID, request.Code)
	},

	"auth.login.complete": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[transactionRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("transactionId", request.TransactionID); err != nil {
			return nil, err
		}
		return engine.LoginComplete(ctx, request.TransactionID)
	},

	"auth.token.refresh": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[refreshRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("refreshToken", request.RefreshToken); err != nil {
			return nil, err
		}
		if err := required("clientId", request.ClientID); err != nil {
			return nil, err
		}
		return engine.Refresh(ctx, request.RefreshToken, request.ClientID)
	},

	"auth.token.authenticate": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[tokenRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		return engine.Authenticate(ctx, request.Token)
	},

	"auth.request.authorize": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[authorizeRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("action", request.Action); err != nil {
			return nil, err
		}
		switch {
		case request.Mailbox != "":
			return engine.AuthorizeMailbox(ctx, request.Token, request.Action, request.Mailbox)
		case request.Domain != "":
			return engine.AuthorizeDomain(ctx, request.Token, request.Action, request.Domain)
		case request.Resource != "":
			return engine.Authorize(ctx, request.Token, request.Action, request.Resource)
		default:
			return nil, Failure{
				Code:    CodeBadRequest,
				Message: "one of mailbox, domain, or resource is required",
			}
		}
	},

	"auth.logout": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[logoutRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("idToken", request.IDToken); err != nil {
			return nil, err
		}
		if err := engine.Logout(ctx, request.IDToken); err != nil {
			return nil, err
		}
		return map[string]bool{"loggedOut": true}, nil
	},

	"auth.session.revoke": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[revokeRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		// A caller may only end its own session here. Ending someone else's is
		// an administrative act with its own operation.
		principal, err := engine.Authenticate(ctx, request.Token)
		if err != nil {
			return nil, err
		}
		if err := engine.RevokeSession(ctx, principal.SessionID, request.Reason); err != nil {
			return nil, err
		}
		return map[string]bool{"revoked": true}, nil
	},

	"auth.credential.passkey.begin": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[stepUpRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("stepUpTransactionId", request.StepUpTransactionID); err != nil {
			return nil, err
		}
		return engine.PasskeyRegisterBegin(ctx, request.Token, request.StepUpTransactionID)
	},

	"auth.credential.passkey.finish": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[passkeyRegisterRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		attestation, err := decodeBase64("attestationObject", request.AttestationObject)
		if err != nil {
			return nil, err
		}
		clientData, err := decodeBase64("clientDataJSON", request.ClientDataJSON)
		if err != nil {
			return nil, err
		}
		return engine.PasskeyRegisterFinish(ctx, request.Token, attestation, clientData)
	},

	"auth.credential.passkey.list": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[tokenRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		passkeys, err := engine.PasskeyList(ctx, request.Token)
		if err != nil {
			return nil, err
		}
		return map[string]any{"passkeys": passkeys}, nil
	},

	"auth.credential.passkey.remove": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[passkeyRemoveRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("stepUpTransactionId", request.StepUpTransactionID); err != nil {
			return nil, err
		}
		if err := required("credentialId", request.CredentialID); err != nil {
			return nil, err
		}
		if err := engine.PasskeyRemove(
			ctx, request.Token, request.StepUpTransactionID, request.CredentialID,
		); err != nil {
			return nil, err
		}
		return map[string]bool{"removed": true}, nil
	},

	"auth.credential.totp.enroll": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[stepUpRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("stepUpTransactionId", request.StepUpTransactionID); err != nil {
			return nil, err
		}
		return engine.TOTPEnroll(ctx, request.Token, request.StepUpTransactionID)
	},

	"auth.credential.totp.activate": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[activateRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("code", request.Code); err != nil {
			return nil, err
		}
		if err := engine.TOTPActivate(ctx, request.Token, request.Code); err != nil {
			return nil, err
		}
		return map[string]bool{"activated": true}, nil
	},

	"auth.credential.recovery.issue": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[stepUpRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("stepUpTransactionId", request.StepUpTransactionID); err != nil {
			return nil, err
		}
		return engine.RecoveryCodesIssue(ctx, request.Token, request.StepUpTransactionID)
	},

	"auth.credential.password.set": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[setPasswordRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("token", request.Token); err != nil {
			return nil, err
		}
		if err := required("stepUpTransactionId", request.StepUpTransactionID); err != nil {
			return nil, err
		}
		if err := required("password", request.Password); err != nil {
			return nil, err
		}
		if err := engine.SetPassword(
			ctx, request.Token, request.StepUpTransactionID, request.Password,
		); err != nil {
			return nil, err
		}
		return map[string]bool{"passwordSet": true}, nil
	},

	"auth.provision.domain": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[provisionDomainRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("domain", request.Domain); err != nil {
			return nil, err
		}
		return engine.ProvisionDomain(ctx, request.Domain)
	},

	"auth.provision.mailbox": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[provisionMailboxRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("address", request.Address); err != nil {
			return nil, err
		}
		return engine.ProvisionMailbox(ctx, request.Address)
	},

	"auth.provision.role": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decodeDomainRole(payload)
		if err != nil {
			return nil, err
		}
		return engine.GrantDomainRole(ctx, request.Address, request.Domain, request.Role)
	},

	"auth.provision.role.revoke": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decodeDomainRole(payload)
		if err != nil {
			return nil, err
		}
		return engine.RevokeDomainRole(ctx, request.Address, request.Domain, request.Role)
	},

	"auth.provision.suspend": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[provisionMailboxRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("address", request.Address); err != nil {
			return nil, err
		}
		if err := engine.SuspendMailbox(ctx, request.Address); err != nil {
			return nil, err
		}
		return map[string]bool{"suspended": true}, nil
	},

	"auth.test.bootstrap": func(ctx context.Context, engine *Engine, payload json.RawMessage) (any, error) {
		request, err := decode[testBootstrapRequest](payload)
		if err != nil {
			return nil, err
		}
		if err := required("address", request.Address); err != nil {
			return nil, err
		}
		if err := required("password", request.Password); err != nil {
			return nil, err
		}
		return engine.TestBootstrap(ctx, request.Address, request.Password, request.Role)
	},

	"auth.jwks": func(ctx context.Context, engine *Engine, _ json.RawMessage) (any, error) {
		if err := engine.refreshKeys(ctx); err != nil {
			return nil, err
		}
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		keys, err := engine.client.SigningKeys(callCtx)
		if err != nil {
			return nil, fmt.Errorf("read signing keys: %w", err)
		}
		return keys, nil
	},
}

// auth.status is registered separately because it reports the size of the
// handler table, and a map literal may not refer to itself.
func init() {
	handlers["auth.status"] = func(ctx context.Context, engine *Engine, _ json.RawMessage) (any, error) {
		callCtx, cancel := engine.call(ctx)
		defer cancel()
		status, reason, err := engine.client.Readiness(callCtx)
		if err != nil {
			return nil, fmt.Errorf("read readiness: %w", err)
		}
		return map[string]any{
			"status":     status,
			"reason":     reason,
			"version":    engine.info.Version,
			"issuer":     engine.options.Issuer,
			"policy":     engine.options.Policy,
			"operations": len(handlers),
		}, nil
	}
}
