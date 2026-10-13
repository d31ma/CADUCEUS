package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// requiredOperations is the surface CADUCEUS actually calls. Start refuses an
// engine that cannot route all of them, so a capability gap is a startup
// failure rather than a mid-login one.
var requiredOperations = []string{
	"authenticator.passkey_register_begin",
	"authenticator.passkey_register_finish",
	"authenticator.passkey_list",
	"authenticator.passkey_remove",
	"authenticator.recovery_codes_issue",
	"authenticator.set_password",
	"authenticator.totp_activate",
	"authenticator.totp_enroll",
	"authn.begin",
	"authn.complete",
	"authn.passkey_options",
	"authn.verify_passkey",
	"authn.verify_password",
	"authn.verify_recovery_code",
	"authn.verify_totp",
	"authorize.decide",
	"grant.create",
	"group.create",
	"group.member_add",
	"group.member_remove",
	"oidc.authorize",
	"oidc.interaction_complete",
	"oidc.introspect",
	"oidc.logout",
	"oidc.token",
	"oidc_client.register",
	"oidc_client.get",
	"principal.create",
	"principal.get",
	"principal.suspend",
	"role.create",
	"session.revoke",
	"session.verify",
	"tenant.bootstrap",
	"tenant.get",
	"token.jwks",
}

// Options configures the engine this process owns.
type Options struct {
	// Binary is the qualified SESAME release executable.
	Binary string
	// Deployment is the directory created by `sesame init`. It holds the
	// signing, secrets, and snapshot keys, and SESAME's own FYLO root, which
	// is never the mail root: one FYLO process owns one root, so the identity
	// ledger and the mail store are separate writers over separate data.
	Deployment string
	// ExpectedVersion is the release identity the runtime lock pins.
	ExpectedVersion string
	// Issuer is the https base URL tokens are issued under. It must match the
	// issuer recorded in the deployment.
	Issuer string
	// RedirectURI is the exact URI registered for the first-party client.
	// CADUCEUS completes interactions itself and never sends a browser there,
	// but SESAME matches redirect URIs exactly, so it must be registered.
	RedirectURI string
	// RegistryFile stores what this deployment has provisioned: the
	// per-tenant first-party client registrations and the identifiers of the
	// roles it created. It holds client secrets and is written 0600.
	RegistryFile string
	// Policy selects the authentication ladder. Both modes require MFA
	// assurance; see policy.go.
	Policy string
	// SessionLifetime bounds an issued session.
	SessionLifetime time.Duration
	// RequestTimeout bounds one engine operation.
	RequestTimeout time.Duration
	// AllowTestOperations exposes auth.test.bootstrap, which provisions a
	// mailbox and hands back its first credentials in one call. It exists so a
	// test suite can obtain a real token from the real engine instead of
	// minting one, and it is refused in production configuration.
	AllowTestOperations bool
	// Stderr receives engine diagnostics.
	Stderr io.Writer
}

// Engine is the one owner of the SESAME process.
type Engine struct {
	options Options
	client  *sesame.Client
	info    sesame.Info
	ledger  *factorLedger

	registryMu sync.Mutex
	clients    map[string]clientRegistration
	roles      map[string]string
	groups     map[string]string

	tenantsMu sync.RWMutex
	tenants   map[string]string

	keysMu    sync.RWMutex
	keys      map[string]verifyingKey
	keysFetch time.Time
}

type clientRegistration struct {
	ClientID string `json:"client_id"`
	Secret   string `json:"client_secret"`
}

// registryFile is the deployment's record of what it has created. SESAME roles
// are immutable and tenant-unique, and the machine protocol has no operation to
// read one back by name, so an identifier that is not recorded here cannot be
// recovered. Losing this file does not lose the roles; it loses the ability to
// grant them again.
type registryFile struct {
	Version int                           `json:"version"`
	Clients map[string]clientRegistration `json:"clients"`
	Roles   map[string]string             `json:"roles"`
	Groups  map[string]string             `json:"groups"`
}

// Start launches, qualifies, and validates the exclusive SESAME owner.
func Start(ctx context.Context, options Options) (*Engine, error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	client, err := sesame.Start(ctx, sesame.Options{
		Binary:     normalized.Binary,
		Deployment: normalized.Deployment,
		Stderr:     normalized.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("start SESAME engine: %w", err)
	}
	engine := &Engine{
		options: normalized,
		client:  client,
		ledger:  newFactorLedger(),
		clients: map[string]clientRegistration{},
		roles:   map[string]string{},
		groups:  map[string]string{},
		tenants: map[string]string{},
		keys:    map[string]verifyingKey{},
	}
	if err := engine.qualify(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := engine.loadRegistry(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return engine, nil
}

func normalizeOptions(options Options) (Options, error) {
	if strings.TrimSpace(options.Binary) == "" {
		return Options{}, errors.New("SESAME binary is required")
	}
	if strings.TrimSpace(options.Deployment) == "" {
		return Options{}, errors.New("SESAME deployment directory is required")
	}
	deployment, err := filepath.Abs(options.Deployment)
	if err != nil {
		return Options{}, fmt.Errorf("resolve SESAME deployment: %w", err)
	}
	options.Deployment = filepath.Clean(deployment)
	if info, err := os.Stat(options.Deployment); err != nil || !info.IsDir() {
		return Options{}, fmt.Errorf(
			"SESAME deployment %q does not exist; run `caduceus auth init` before starting",
			options.Deployment,
		)
	}
	// The engine refuses anything but an absolute https issuer, so accepting
	// more here would only move the failure from startup into `auth init`.
	if !strings.HasPrefix(options.Issuer, "https://") {
		return Options{}, errors.New("SESAME_ISSUER must be an absolute https URL")
	}
	if strings.TrimSpace(options.RedirectURI) == "" {
		return Options{}, errors.New("SESAME redirect URI is required")
	}
	if options.RegistryFile == "" {
		options.RegistryFile = filepath.Join(options.Deployment, "caduceus-registry.json")
	}
	if options.SessionLifetime <= 0 {
		options.SessionLifetime = 12 * time.Hour
	}
	if options.SessionLifetime < time.Minute || options.SessionLifetime > 30*24*time.Hour {
		return Options{}, errors.New("SESAME_SESSION_LIFETIME must be between 1m and 720h")
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = 10 * time.Second
	}
	if err := ValidatePolicy(options.Policy); err != nil {
		return Options{}, err
	}
	if options.Policy == "" {
		options.Policy = PolicyPasskeyFirst
	}
	return options, nil
}

// qualify proves the running engine is the pinned release, speaks the protocol
// this client speaks, routes every operation CADUCEUS calls, and has durable
// storage. A gap in any of them is a startup failure.
func (engine *Engine) qualify(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, engine.options.RequestTimeout)
	defer cancel()

	info, err := engine.client.Version(ctx)
	if err != nil {
		return fmt.Errorf("read SESAME identity: %w", err)
	}
	if expected := engine.options.ExpectedVersion; expected != "" && info.Version != expected {
		return fmt.Errorf("SESAME release is %s but the runtime lock pins %s", info.Version, expected)
	}
	if info.ProtocolVersion != sesame.ProtocolVersion {
		return fmt.Errorf(
			"SESAME speaks machine protocol %s but this client speaks %s",
			info.ProtocolVersion, sesame.ProtocolVersion,
		)
	}
	if err := engine.client.RequireOperations(ctx, requiredOperations...); err != nil {
		return fmt.Errorf("qualify SESAME operations: %w", err)
	}
	status, reason, err := engine.client.Readiness(ctx)
	if err != nil {
		return fmt.Errorf("read SESAME readiness: %w", err)
	}
	// The engine reports "ok" when storage is configured and its ledger has
	// replayed, and "not_ready" with a reason code otherwise.
	if status != "ok" && status != "ready" {
		return fmt.Errorf("SESAME is not ready: %s (%s)", status, reason)
	}
	engine.info = info
	return nil
}

// Info returns the qualified engine identity.
func (engine *Engine) Info() sesame.Info {
	return engine.info
}

// Close stops the engine.
func (engine *Engine) Close() error {
	if engine == nil || engine.client == nil {
		return nil
	}
	return engine.client.Close()
}

func (engine *Engine) call(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, engine.options.RequestTimeout)
}

// TenantID returns this deployment's one tenant, creating it on first use.
//
// Every mailbox in every mail domain is a principal in it. Domains are
// resources, not tenants; see TenantName for why.
func (engine *Engine) TenantID(ctx context.Context) (string, error) {
	engine.tenantsMu.RLock()
	cached, found := engine.tenants[TenantName]
	engine.tenantsMu.RUnlock()
	if found {
		return cached, nil
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.TenantBootstrap(callCtx, TenantName)
	if err != nil {
		return "", fmt.Errorf("bootstrap deployment tenant: %w", err)
	}
	engine.tenantsMu.Lock()
	engine.tenants[TenantName] = result.Tenant.ID
	engine.tenantsMu.Unlock()
	return result.Tenant.ID, nil
}

// client registration ---------------------------------------------------

func (engine *Engine) loadRegistry() error {
	data, err := os.ReadFile(engine.options.RegistryFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read provisioning registry: %w", err)
	}
	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("decode provisioning registry: %w", err)
	}
	if file.Version != 1 {
		return fmt.Errorf("provisioning registry version %d is unsupported", file.Version)
	}
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	for tenant, registration := range file.Clients {
		engine.clients[tenant] = registration
	}
	for key, roleID := range file.Roles {
		engine.roles[key] = roleID
	}
	for key, groupID := range file.Groups {
		engine.groups[key] = groupID
	}
	return nil
}

func (engine *Engine) saveRegistryLocked() error {
	file := registryFile{
		Version: 1,
		Clients: engine.clients,
		Roles:   engine.roles,
		Groups:  engine.groups,
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode provisioning registry: %w", err)
	}
	temporary := engine.options.RegistryFile + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write provisioning registry: %w", err)
	}
	if err := os.Rename(temporary, engine.options.RegistryFile); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit provisioning registry: %w", err)
	}
	return nil
}

// rememberRole records one created role identifier so it can be granted again
// after this process exits.
func (engine *Engine) rememberRole(tenantID, name, roleID string) error {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	engine.roles[tenantID+"/"+name] = roleID
	return engine.saveRegistryLocked()
}

// recallRole returns a previously created role identifier.
func (engine *Engine) recallRole(tenantID, name string) (string, bool) {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	roleID, found := engine.roles[tenantID+"/"+name]
	return roleID, found
}

// rememberGroup and recallGroup do for groups what the pair above does for
// roles, and for the same reason: the machine protocol has no operation that
// reads a group back by name.
func (engine *Engine) rememberGroup(tenantID, name, groupID string) error {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	engine.groups[tenantID+"/"+name] = groupID
	return engine.saveRegistryLocked()
}

func (engine *Engine) recallGroup(tenantID, name string) (string, bool) {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	groupID, found := engine.groups[tenantID+"/"+name]
	return groupID, found
}

// clientFor returns the first-party OIDC client for one tenant, registering it
// on first use. The secret exists once, at registration, so it is persisted
// beside the deployment keys rather than regenerated per start: rotating it on
// every restart would invalidate every refresh family in flight.
func (engine *Engine) clientFor(ctx context.Context, tenantID string) (clientRegistration, error) {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	if registration, found := engine.clients[tenantID]; found {
		return registration, nil
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	registered, err := engine.client.ClientRegister(
		callCtx,
		tenantID,
		"caduceus",
		"confidential",
		[]string{engine.options.RedirectURI},
		[]string{"openid", "offline_access"},
		"first_party",
		[]string{engine.options.RedirectURI},
	)
	if err != nil {
		return clientRegistration{}, fmt.Errorf("register first-party client: %w", err)
	}
	registration := clientRegistration{ClientID: registered.Client.ID, Secret: registered.Secret}
	engine.clients[tenantID] = registration
	if err := engine.saveRegistryLocked(); err != nil {
		// A registration that cannot be persisted is unusable: the next start
		// would register a second client and orphan every refresh family
		// issued against this one.
		delete(engine.clients, tenantID)
		return clientRegistration{}, err
	}
	return registration, nil
}

// clientByID returns the registration one access token's audience belongs to.
func (engine *Engine) clientByID(clientID string) (clientRegistration, bool) {
	engine.registryMu.Lock()
	defer engine.registryMu.Unlock()
	for _, registration := range engine.clients {
		if registration.ClientID == clientID {
			return registration, true
		}
	}
	return clientRegistration{}, false
}
