// Package config loads and validates the versioned CADUCEUS runtime contract.
package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultPort                   = "8080"
	defaultMetricsHost            = "127.0.0.1"
	defaultMetricsPort            = "9090"
	defaultStaticDir              = "build/static"
	defaultSchemaDir              = "db/schemas"
	defaultRuntimeLock            = "runtime.lock.json"
	defaultShutdownTimeout        = "10s"
	defaultFYLOBridge             = "127.0.0.1:9876"
	defaultFYLOTimeout            = "10s"
	defaultExternalAccountTimeout = "15s"
	defaultSessionLifetime        = "12h"
	defaultSESAMETimeout          = "10s"
	defaultAuthPolicy             = "passkey-first"
)

// Config mirrors the uppercase v1 deployment keys. Business-feature values
// remain present during migration so one file can configure both runtimes;
// Phase 1 consumes only the runtime fields.
type Config struct {
	Host                             string `json:"HOST"`
	Port                             string `json:"PORT"`
	MetricsHost                      string `json:"METRICS_HOST"`
	MetricsPort                      string `json:"METRICS_PORT"`
	StaticDir                        string `json:"STATIC_DIR"`
	RuntimeLock                      string `json:"RUNTIME_LOCK"`
	ShutdownTimeout                  string `json:"SHUTDOWN_TIMEOUT"`
	PublicURL                        string `json:"PUBLIC_URL"`
	FYLORoot                         string `json:"FYLO_ROOT"`
	FYLOSchema                       string `json:"FYLO_SCHEMA"`
	FYLOBinary                       string `json:"FYLO_BINARY"`
	FYLOPosixAccess                  string `json:"FYLO_POSIX_ACCESS"`
	FYLOBridgeAddress                string `json:"FYLO_BRIDGE_ADDRESS"`
	FYLOBridgeSecret                 string `json:"FYLO_BRIDGE_SECRET"`
	FYLOIncomingRoot                 string `json:"FYLO_INCOMING_ROOT"`
	FYLORequestTimeout               string `json:"FYLO_REQUEST_TIMEOUT"`
	FYLOMaxRequestBytes              string `json:"FYLO_MAX_REQUEST_BYTES"`
	FYLOMaxResponseBytes             string `json:"FYLO_MAX_RESPONSE_BYTES"`
	FYLORestartLimit                 string `json:"FYLO_RESTART_LIMIT"`
	FYLOMaxAttachmentBytes           string `json:"FYLO_MAX_ATTACHMENT_BYTES"`
	FYLOAttachmentQuota              string `json:"FYLO_ATTACHMENT_QUOTA_BYTES"`
	ExternalAccountCredentialKeys    string `json:"EXTERNAL_ACCOUNT_CREDENTIAL_KEYS"`
	ExternalAccountActiveKey         string `json:"EXTERNAL_ACCOUNT_ACTIVE_KEY"`
	ExternalAccountConnectionTimeout string `json:"EXTERNAL_ACCOUNT_CONNECTION_TIMEOUT"`
	ExternalAccountPrivateNetworks   string `json:"EXTERNAL_ACCOUNT_PRIVATE_CIDRS"`
	MicrosoftOAuthClientID           string `json:"MICROSOFT_OAUTH_CLIENT_ID"`
	MicrosoftOAuthClientSecret       string `json:"MICROSOFT_OAUTH_CLIENT_SECRET"`
	MicrosoftOAuthTenant             string `json:"MICROSOFT_OAUTH_TENANT"`
	MicrosoftOAuthRedirectURI        string `json:"MICROSOFT_OAUTH_REDIRECT_URI"`
	GoogleOAuthClientID              string `json:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecret          string `json:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleOAuthRedirectURI           string `json:"GOOGLE_OAUTH_REDIRECT_URI"`
	SESAMEBinary                     string `json:"SESAME_BINARY"`
	SESAMEDeployment                 string `json:"SESAME_DEPLOYMENT"`
	SESAMEIssuer                     string `json:"SESAME_ISSUER"`
	SESAMERedirectURI                string `json:"SESAME_REDIRECT_URI"`
	SESAMESessionLifetime            string `json:"SESAME_SESSION_LIFETIME"`
	SESAMERequestTimeout             string `json:"SESAME_REQUEST_TIMEOUT"`
	InboundWebhookSecret             string `json:"INBOUND_WEBHOOK_SECRET"`
	AuthPolicy                       string `json:"CADUCEUS_AUTH_POLICY"`
	MailServerEnabled                string `json:"MAIL_SERVER_ENABLED"`
	MailHostname                     string `json:"MAIL_HOSTNAME"`
	SMTPInboundHost                  string `json:"SMTP_INBOUND_HOST"`
	SMTPInboundPort                  string `json:"SMTP_INBOUND_PORT"`
	SMTPMaxSize                      string `json:"SMTP_MAX_SIZE"`
	MailAuthVerify                   string `json:"MAIL_AUTH_VERIFY"`
	MailSubmissionEnabled            string `json:"MAIL_SUBMISSION_ENABLED"`
	SMTPSubmissionHost               string `json:"SMTP_SUBMISSION_HOST"`
	SMTPSubmissionPort               string `json:"SMTP_SUBMISSION_PORT"`
	SMTPSubmissionTLSPort            string `json:"SMTP_SUBMISSION_TLS_PORT"`
	SMTPAdapter                      string `json:"SMTP_ADAPTER"`
	SMTPTLSPolicy                    string `json:"SMTP_TLS_POLICY"`
	SMTPFrom                         string `json:"SMTP_FROM"`
	DKIMDomain                       string `json:"DKIM_DOMAIN"`
	DKIMSelector                     string `json:"DKIM_SELECTOR"`
	DKIMPrivateKey                   string `json:"DKIM_PRIVATE_KEY"`
	DKIMPrivateKeyFile               string `json:"DKIM_PRIVATE_KEY_FILE"`
	MailTLSCert                      string `json:"MAIL_TLS_CERT"`
	MailTLSKey                       string `json:"MAIL_TLS_KEY"`
	MailTLSCertFile                  string `json:"MAIL_TLS_CERT_FILE"`
	MailTLSKeyFile                   string `json:"MAIL_TLS_KEY_FILE"`
	MailTLSDir                       string `json:"MAIL_TLS_DIR"`
	MailACMEEnabled                  string `json:"MAIL_ACME_ENABLED"`
	MailACMEEmail                    string `json:"MAIL_ACME_EMAIL"`
	MailACMEStaging                  string `json:"MAIL_ACME_STAGING"`
	MailACMEHTTPPort                 string `json:"MAIL_ACME_HTTP_PORT"`
	MailACMEAccountFile              string `json:"MAIL_ACME_ACCOUNT_FILE"`
	OutboundQueueWorker              string `json:"OUTBOUND_QUEUE_WORKER"`
	OutboundQueueIntervalMS          string `json:"OUTBOUND_QUEUE_INTERVAL_MS"`
	OutboundQueueTimeoutMS           string `json:"OUTBOUND_QUEUE_TIMEOUT_MS"`
	SnoozeWorker                     string `json:"SNOOZE_WORKER"`
	SnoozeIntervalMS                 string `json:"SNOOZE_INTERVAL_MS"`
	SnoozeTimeoutMS                  string `json:"SNOOZE_TIMEOUT_MS"`
	ReminderWorker                   string `json:"REMINDER_WORKER"`
	ReminderIntervalMS               string `json:"REMINDER_INTERVAL_MS"`
	ReminderTimeoutMS                string `json:"REMINDER_TIMEOUT_MS"`
	MailStatusIntervalMS             string `json:"MAIL_STATUS_INTERVAL_MS"`
	SpamFilterDisabled               string `json:"SPAM_FILTER_DISABLED"`
	SpamScoreThreshold               string `json:"SPAM_SCORE_THRESHOLD"`
	WebPushDisabled                  string `json:"WEB_PUSH_DISABLED"`
	VAPIDPublicKey                   string `json:"VAPID_PUBLIC_KEY"`
	VAPIDPrivateKey                  string `json:"VAPID_PRIVATE_KEY"`
	VAPIDSubject                     string `json:"VAPID_SUBJECT"`
	StreamMaxMS                      string `json:"STREAM_MAX_MS"`
	StreamPollMS                     string `json:"STREAM_POLL_MS"`
	StreamReplayLimit                string `json:"STREAM_REPLAY_LIMIT"`
	NodeEnv                          string `json:"NODE_ENV"`
	EnableTestRoutes                 string `json:"CADUCEUS_ENABLE_TEST_ROUTES"`

	sourceDir string
}

// Defaults returns the loopback-first Phase 1 defaults.
func Defaults() Config {
	return Config{
		Host:                             defaultHost,
		Port:                             defaultPort,
		MetricsHost:                      defaultMetricsHost,
		MetricsPort:                      defaultMetricsPort,
		StaticDir:                        defaultStaticDir,
		RuntimeLock:                      defaultRuntimeLock,
		ShutdownTimeout:                  defaultShutdownTimeout,
		FYLOSchema:                       defaultSchemaDir,
		FYLOBridgeAddress:                defaultFYLOBridge,
		FYLORequestTimeout:               defaultFYLOTimeout,
		FYLOMaxRequestBytes:              "33554432",
		FYLOMaxResponseBytes:             "33554432",
		FYLORestartLimit:                 "3",
		FYLOMaxAttachmentBytes:           "52428800",
		FYLOAttachmentQuota:              "10737418240",
		ExternalAccountConnectionTimeout: defaultExternalAccountTimeout,
		MicrosoftOAuthTenant:             "organizations",
		SESAMESessionLifetime:            defaultSessionLifetime,
		SESAMERequestTimeout:             defaultSESAMETimeout,
		AuthPolicy:                       defaultAuthPolicy,
		SMTPAdapter:                      "direct",
		SMTPTLSPolicy:                    "opportunistic",
		OutboundQueueWorker:              "on",
		OutboundQueueIntervalMS:          "30000",
		OutboundQueueTimeoutMS:           "60000",
		SnoozeWorker:                     "on",
		SnoozeIntervalMS:                 "30000",
		SnoozeTimeoutMS:                  "60000",
		ReminderWorker:                   "on",
		ReminderIntervalMS:               "30000",
		ReminderTimeoutMS:                "60000",
	}
}

// Load reads an optional strict JSON file, applies known environment
// overrides, resolves filesystem paths, and validates the result.
func Load(path string) (Config, error) {
	cfg := Defaults()
	cfg.sourceDir = currentDirectory()
	if path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Config{}, fmt.Errorf("resolve config path: %w", err)
		}
		data, err := os.ReadFile(absolute)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if err := decodeStrict(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
		cfg.sourceDir = filepath.Dir(absolute)
	}
	applyEnvironment(&cfg)
	cfg.resolvePaths()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func decodeStrict(data []byte, target *Config) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("configuration must contain one JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("configuration contains trailing JSON")
	}
	return nil
}

func applyEnvironment(cfg *Config) {
	value := reflect.ValueOf(cfg).Elem()
	typ := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := typ.Field(index)
		key := field.Tag.Get("json")
		if key == "" || key == "-" || !value.Field(index).CanSet() || value.Field(index).Kind() != reflect.String {
			continue
		}
		if environmentValue, ok := os.LookupEnv(key); ok {
			value.Field(index).SetString(environmentValue)
		}
	}
}

func (cfg *Config) resolvePaths() {
	for _, target := range []*string{
		&cfg.StaticDir, &cfg.RuntimeLock, &cfg.FYLORoot, &cfg.FYLOSchema,
		&cfg.FYLOBinary, &cfg.FYLOIncomingRoot,
		&cfg.SESAMEBinary, &cfg.SESAMEDeployment,
	} {
		if *target == "" || filepath.IsAbs(*target) {
			continue
		}
		*target = filepath.Clean(filepath.Join(cfg.sourceDir, *target))
	}
	if cfg.FYLOIncomingRoot == "" && cfg.FYLORoot != "" {
		cfg.FYLOIncomingRoot = filepath.Join(cfg.FYLORoot, ".caduceus-incoming")
	}
}

// Validate rejects unsafe or malformed runtime values.
func (cfg Config) Validate() error {
	validators := []func() error{
		cfg.validateListeners,
		cfg.validateRequiredValues,
		cfg.validateSecrets,
		cfg.validateDurations,
		cfg.validateExternalAccountNetworks,
		cfg.validateMicrosoftOAuth,
		cfg.validateGoogleOAuth,
		cfg.validateKnownValues,
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) validateExternalAccountNetworks() error {
	_, err := parseExternalAccountPrivateCIDRs(cfg.ExternalAccountPrivateNetworks)
	return err
}

func (cfg Config) MicrosoftOAuthConfigured() bool {
	return strings.TrimSpace(cfg.MicrosoftOAuthClientID) != "" && strings.TrimSpace(cfg.MicrosoftOAuthClientSecret) != "" && strings.TrimSpace(cfg.MicrosoftOAuthRedirectURI) != ""
}

func (cfg Config) GoogleOAuthConfigured() bool {
	return strings.TrimSpace(cfg.GoogleOAuthClientID) != "" && strings.TrimSpace(cfg.GoogleOAuthClientSecret) != "" && strings.TrimSpace(cfg.GoogleOAuthRedirectURI) != ""
}

func (cfg Config) validateGoogleOAuth() error {
	requested := strings.TrimSpace(cfg.GoogleOAuthClientID) != "" || strings.TrimSpace(cfg.GoogleOAuthClientSecret) != "" || strings.TrimSpace(cfg.GoogleOAuthRedirectURI) != ""
	if !requested {
		return nil
	}
	if !cfg.GoogleOAuthConfigured() || len(cfg.GoogleOAuthClientID) > 512 || len(cfg.GoogleOAuthClientSecret) < 16 || len(cfg.GoogleOAuthClientSecret) > 8192 {
		return errors.New("GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, and GOOGLE_OAUTH_REDIRECT_URI must be configured together")
	}
	if err := validateOAuthRedirect(cfg.GoogleOAuthRedirectURI); err != nil {
		return errors.New("GOOGLE_OAUTH_REDIRECT_URI must be an absolute HTTPS callback URL without query or fragment, except on an explicit loopback address")
	}
	return nil
}

func (cfg Config) validateMicrosoftOAuth() error {
	requested := strings.TrimSpace(cfg.MicrosoftOAuthClientID) != "" || strings.TrimSpace(cfg.MicrosoftOAuthClientSecret) != "" || strings.TrimSpace(cfg.MicrosoftOAuthRedirectURI) != ""
	if !requested {
		return nil
	}
	if !cfg.MicrosoftOAuthConfigured() || len(cfg.MicrosoftOAuthClientID) > 256 || len(cfg.MicrosoftOAuthClientSecret) < 16 || len(cfg.MicrosoftOAuthClientSecret) > 8192 {
		return errors.New("MICROSOFT_OAUTH_CLIENT_ID, MICROSOFT_OAUTH_CLIENT_SECRET, and MICROSOFT_OAUTH_REDIRECT_URI must be configured together")
	}
	tenant := strings.TrimSpace(cfg.MicrosoftOAuthTenant)
	if tenant == "" || len(tenant) > 253 || strings.Contains(tenant, "..") {
		return errors.New("MICROSOFT_OAUTH_TENANT is invalid")
	}
	for _, character := range tenant {
		if character != '-' && character != '.' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return errors.New("MICROSOFT_OAUTH_TENANT is invalid")
		}
	}
	redirect, err := url.Parse(strings.TrimSpace(cfg.MicrosoftOAuthRedirectURI))
	if err != nil || !redirect.IsAbs() || redirect.User != nil || redirect.Hostname() == "" || redirect.RawQuery != "" || redirect.Fragment != "" {
		return errors.New("MICROSOFT_OAUTH_REDIRECT_URI must be an absolute callback URL without query or fragment")
	}
	if redirect.Scheme != "https" {
		ip := net.ParseIP(redirect.Hostname())
		if redirect.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return errors.New("MICROSOFT_OAUTH_REDIRECT_URI must use HTTPS except on an explicit loopback address")
		}
	}
	return nil
}

func validateOAuthRedirect(value string) error {
	redirect, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !redirect.IsAbs() || redirect.User != nil || redirect.Hostname() == "" || redirect.RawQuery != "" || redirect.Fragment != "" {
		return errors.New("invalid redirect URI")
	}
	if redirect.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(redirect.Hostname())
	if redirect.Scheme != "http" || ip == nil || !ip.IsLoopback() {
		return errors.New("redirect URI requires HTTPS")
	}
	return nil
}

// ExternalAccountPrivateCIDRs returns the operator-approved private networks
// in canonical form. Validation has already rejected broad, public, link-local,
// multicast, and unspecified ranges.
func (cfg Config) ExternalAccountPrivateCIDRs() []string {
	networks, _ := parseExternalAccountPrivateCIDRs(cfg.ExternalAccountPrivateNetworks)
	return networks
}

func parseExternalAccountPrivateCIDRs(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 32 {
		return nil, errors.New("EXTERNAL_ACCOUNT_PRIVATE_CIDRS accepts at most 32 networks")
	}
	allowed := privateNetworkEnvelopes()
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		ip, network, err := net.ParseCIDR(candidate)
		if err != nil || ip.String() != network.IP.String() || candidate != network.String() || !containedByAny(network, allowed) {
			return nil, errors.New("EXTERNAL_ACCOUNT_PRIVATE_CIDRS must contain canonical private or loopback CIDRs")
		}
		if _, duplicate := seen[candidate]; duplicate {
			return nil, errors.New("EXTERNAL_ACCOUNT_PRIVATE_CIDRS must not contain duplicates")
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	return result, nil
}

func privateNetworkEnvelopes() []*net.IPNet {
	values := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "fc00::/7", "::1/128"}
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, _ := net.ParseCIDR(value)
		result = append(result, network)
	}
	return result
}

func containedByAny(candidate *net.IPNet, allowed []*net.IPNet) bool {
	last := append(net.IP(nil), candidate.IP...)
	for index := range last {
		last[index] |= ^candidate.Mask[index]
	}
	for _, envelope := range allowed {
		if envelope.Contains(candidate.IP) && envelope.Contains(last) {
			return true
		}
	}
	return false
}

func (cfg Config) validateListeners() error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("HOST must not be empty")
	}
	if _, err := ParsePort("PORT", cfg.Port); err != nil {
		return err
	}
	if !isLoopbackHost(cfg.MetricsHost) {
		return errors.New("METRICS_HOST must resolve to a loopback address")
	}
	bridgeHost, bridgePort, err := net.SplitHostPort(cfg.FYLOBridgeAddress)
	if err != nil || !isBridgeLoopback(bridgeHost) {
		return errors.New("FYLO_BRIDGE_ADDRESS must be an explicit loopback address and port")
	}
	if _, err := ParsePort("FYLO_BRIDGE_ADDRESS port", bridgePort); err != nil {
		return errors.New("FYLO_BRIDGE_ADDRESS must be an explicit loopback address and port")
	}
	if cfg.FYLOBridgeSecret != "" && len(cfg.FYLOBridgeSecret) < 32 {
		return errors.New("FYLO_BRIDGE_SECRET must contain at least 32 characters")
	}
	if _, err := ParsePort("METRICS_PORT", cfg.MetricsPort); err != nil {
		return err
	}
	if net.JoinHostPort(cfg.Host, cfg.Port) == net.JoinHostPort(cfg.MetricsHost, cfg.MetricsPort) {
		return errors.New("HTTP and metrics listeners must use different addresses")
	}
	return nil
}

func (cfg Config) validateRequiredValues() error {
	required := []struct {
		name  string
		value string
	}{
		{name: "STATIC_DIR", value: cfg.StaticDir},
		{name: "RUNTIME_LOCK", value: cfg.RuntimeLock},
		{name: "FYLO_BINARY", value: cfg.FYLOBinary},
		{name: "FYLO_ROOT", value: cfg.FYLORoot},
		{name: "FYLO_SCHEMA", value: cfg.FYLOSchema},
		{name: "SESAME_BINARY", value: cfg.SESAMEBinary},
		{name: "SESAME_DEPLOYMENT", value: cfg.SESAMEDeployment},
		{name: "SESAME_ISSUER", value: cfg.SESAMEIssuer},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	return nil
}

func (cfg Config) validateSecrets() error {
	secrets := []struct {
		name  string
		value string
	}{
		{name: "INBOUND_WEBHOOK_SECRET", value: cfg.InboundWebhookSecret},
	}
	for _, secret := range secrets {
		if len(secret.value) < 16 {
			return fmt.Errorf("%s must contain at least 16 characters", secret.name)
		}
	}
	if cfg.ExternalAccountCredentialKeys != "" || cfg.ExternalAccountActiveKey != "" {
		if _, _, err := cfg.ExternalAccountKeyring(); err != nil {
			return err
		}
	}
	return nil
}

// ExternalAccountKeyring decodes the versioned AES-256 keys used only for
// external-provider credentials. Key material is never included in errors.
func (cfg Config) ExternalAccountKeyring() (uint32, map[uint32][]byte, error) {
	if strings.TrimSpace(cfg.ExternalAccountCredentialKeys) == "" || strings.TrimSpace(cfg.ExternalAccountActiveKey) == "" {
		return 0, nil, errors.New("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS and EXTERNAL_ACCOUNT_ACTIVE_KEY are required")
	}
	active64, err := strconv.ParseUint(cfg.ExternalAccountActiveKey, 10, 32)
	if err != nil || active64 == 0 || strconv.FormatUint(active64, 10) != cfg.ExternalAccountActiveKey {
		return 0, nil, errors.New("EXTERNAL_ACCOUNT_ACTIVE_KEY must be a canonical positive key version")
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(cfg.ExternalAccountCredentialKeys), &encoded); err != nil || len(encoded) == 0 {
		return 0, nil, errors.New("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS must be a non-empty JSON object")
	}
	keys := make(map[uint32][]byte, len(encoded))
	for versionText, material := range encoded {
		version64, parseErr := strconv.ParseUint(versionText, 10, 32)
		if parseErr != nil || version64 == 0 || strconv.FormatUint(version64, 10) != versionText {
			return 0, nil, errors.New("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS contains an invalid key version")
		}
		key, decodeErr := decodeCredentialKey(material)
		if decodeErr != nil {
			return 0, nil, errors.New("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS values must be base64-encoded 32-byte keys")
		}
		keys[uint32(version64)] = key
	}
	active := uint32(active64)
	if _, found := keys[active]; !found {
		return 0, nil, errors.New("EXTERNAL_ACCOUNT_ACTIVE_KEY is absent from EXTERNAL_ACCOUNT_CREDENTIAL_KEYS")
	}
	return active, keys, nil
}

func decodeCredentialKey(value string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(value)
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid credential key")
	}
	return key, nil
}

func (cfg Config) validateDurations() error {
	checks := []struct {
		name    string
		value   string
		minimum time.Duration
		maximum time.Duration
	}{
		{"SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout, time.Second, 2 * time.Minute},
		{"FYLO_REQUEST_TIMEOUT", cfg.FYLORequestTimeout, 100 * time.Millisecond, 2 * time.Minute},
		{"SESAME_REQUEST_TIMEOUT", cfg.SESAMERequestTimeout, 100 * time.Millisecond, 2 * time.Minute},
		{"EXTERNAL_ACCOUNT_CONNECTION_TIMEOUT", cfg.ExternalAccountConnectionTimeout, time.Second, 2 * time.Minute},
		{"SESAME_SESSION_LIFETIME", cfg.SESAMESessionLifetime, time.Minute, 720 * time.Hour},
	}
	for _, check := range checks {
		if err := validateDuration(check.name, check.value, check.minimum, check.maximum); err != nil {
			return err
		}
	}
	return nil
}

func validateDuration(name, value string, minimum, maximum time.Duration) error {
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minimum || duration > maximum {
		return fmt.Errorf("%s must be between %s and %s", name, minimum, maximum)
	}
	return nil
}

func (cfg Config) ExternalAccountConnectionDuration() time.Duration {
	duration, _ := time.ParseDuration(cfg.ExternalAccountConnectionTimeout)
	return duration
}

// SESAMERequestDuration returns the validated per-operation deadline.
func (cfg Config) SESAMERequestDuration() time.Duration {
	duration, _ := time.ParseDuration(cfg.SESAMERequestTimeout)
	return duration
}

// SESAMESessionDuration returns the validated session bound.
func (cfg Config) SESAMESessionDuration() time.Duration {
	duration, _ := time.ParseDuration(cfg.SESAMESessionLifetime)
	return duration
}

// SESAMERedirect returns the exact URI registered for the first-party client.
// It is derived from PUBLIC_URL when unset so a single-host deployment
// configures one value rather than two that must agree.
func (cfg Config) SESAMERedirect() string {
	if cfg.SESAMERedirectURI != "" {
		return cfg.SESAMERedirectURI
	}
	if cfg.PublicURL == "" {
		return ""
	}
	return strings.TrimRight(cfg.PublicURL, "/") + "/auth/callback"
}

// ParsePort converts one validated TCP port.
func ParsePort(name, value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be an integer from 1 through 65535", name)
	}
	return port, nil
}

// HTTPAddress returns the public listener address.
func (cfg Config) HTTPAddress() string {
	return net.JoinHostPort(cfg.Host, cfg.Port)
}

// MetricsAddress returns the loopback operations listener address.
func (cfg Config) MetricsAddress() string {
	return net.JoinHostPort(cfg.MetricsHost, cfg.MetricsPort)
}

// ShutdownDuration returns the already validated shutdown bound.
func (cfg Config) ShutdownDuration() time.Duration {
	duration, _ := time.ParseDuration(cfg.ShutdownTimeout)
	return duration
}

// FYLORequestDuration returns the validated per-operation deadline.
func (cfg Config) FYLORequestDuration() time.Duration {
	duration, _ := time.ParseDuration(cfg.FYLORequestTimeout)
	return duration
}

// FYLOInteger returns one already validated positive numeric boundary.
func (cfg Config) FYLOInteger(value string) int {
	number, _ := strconv.Atoi(value)
	return number
}

// FYLOInt64 returns one already validated positive byte boundary.
func (cfg Config) FYLOInt64(value string) int64 {
	number, _ := strconv.ParseInt(value, 10, 64)
	return number
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isBridgeLoopback(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}

func currentDirectory() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return directory
}
