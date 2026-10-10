package config

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalAccountCredentialKeyringParsesVersionedAESKeys(t *testing.T) {
	first := bytes.Repeat([]byte{0x11}, 32)
	second := bytes.Repeat([]byte{0x22}, 32)
	cfg := Defaults()
	cfg.ExternalAccountActiveKey = "2"
	cfg.ExternalAccountCredentialKeys = `{"1":"` + base64.RawURLEncoding.EncodeToString(first) + `","2":"` + base64.RawURLEncoding.EncodeToString(second) + `"}`

	active, keys, err := cfg.ExternalAccountKeyring()
	if err != nil {
		t.Fatalf("ExternalAccountKeyring() error = %v", err)
	}
	if active != 2 || !bytes.Equal(keys[1], first) || !bytes.Equal(keys[2], second) {
		t.Fatalf("ExternalAccountKeyring() = %d, %#v", active, keys)
	}
}

func TestMicrosoftOAuthConfigurationIsOptionalButAtomic(t *testing.T) {
	cfg := validConfig()
	if cfg.MicrosoftOAuthConfigured() {
		t.Fatal("Microsoft OAuth unexpectedly configured by default")
	}
	cfg.MicrosoftOAuthClientID = "client-id"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MICROSOFT_OAUTH") {
		t.Fatalf("partial Microsoft OAuth Validate() error = %v", err)
	}
	cfg.MicrosoftOAuthClientSecret = "client-secret-value"
	cfg.MicrosoftOAuthRedirectURI = "https://mail.example.com/settings"
	cfg.MicrosoftOAuthTenant = "organizations"
	if err := cfg.Validate(); err != nil || !cfg.MicrosoftOAuthConfigured() {
		t.Fatalf("configured Microsoft OAuth = %t, %v", cfg.MicrosoftOAuthConfigured(), err)
	}
}

func TestGoogleOAuthConfigurationIsOptionalButAtomic(t *testing.T) {
	cfg := validConfig()
	if cfg.GoogleOAuthConfigured() {
		t.Fatal("Google OAuth unexpectedly configured by default")
	}
	cfg.GoogleOAuthClientID = "client-id"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "GOOGLE_OAUTH") {
		t.Fatalf("partial Google OAuth Validate() error = %v", err)
	}
	cfg.GoogleOAuthClientSecret = "client-secret-value"
	cfg.GoogleOAuthRedirectURI = "https://mail.example.com/settings"
	if err := cfg.Validate(); err != nil || !cfg.GoogleOAuthConfigured() {
		t.Fatalf("configured Google OAuth = %t, %v", cfg.GoogleOAuthConfigured(), err)
	}
}

func TestValidateRejectsPartialOrMalformedExternalAccountKeyrings(t *testing.T) {
	valid := validConfig()
	encoded := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, 32))
	tests := []struct {
		name   string
		active string
		keys   string
	}{
		{name: "active only", active: "1"},
		{name: "keys only", keys: `{"1":"` + encoded + `"}`},
		{name: "bad active", active: "zero", keys: `{"1":"` + encoded + `"}`},
		{name: "missing active version", active: "2", keys: `{"1":"` + encoded + `"}`},
		{name: "short key", active: "1", keys: `{"1":"c2hvcnQ"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			cfg.ExternalAccountActiveKey = test.active
			cfg.ExternalAccountCredentialKeys = test.keys
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "EXTERNAL_ACCOUNT") {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestExternalAccountPrivateNetworksAreExplicitCanonicalCIDRs(t *testing.T) {
	cfg := validConfig()
	cfg.ExternalAccountPrivateNetworks = "127.0.0.0/8, 10.20.0.0/16, ::1/128"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	networks := cfg.ExternalAccountPrivateCIDRs()
	if strings.Join(networks, ",") != "127.0.0.0/8,10.20.0.0/16,::1/128" {
		t.Fatalf("ExternalAccountPrivateCIDRs() = %#v", networks)
	}

	cfg.ExternalAccountPrivateNetworks = "10.20.1.1/16"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "EXTERNAL_ACCOUNT_PRIVATE_CIDRS") {
		t.Fatalf("Validate(non-canonical CIDR) error = %v", err)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"UNKNOWN":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load error = %v, want unknown field", err)
	}
}

func TestValidateRejectsPublicMetrics(t *testing.T) {
	cfg := validConfig()
	cfg.MetricsHost = "0.0.0.0"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("Validate error = %v, want loopback error", err)
	}
}

func validConfig() Config {
	cfg := Defaults()
	cfg.FYLOBinary = "/tmp/fylo"
	cfg.FYLORoot = "/tmp/root"
	cfg.InboundWebhookSecret = "fedcba9876543210"
	cfg.SESAMEBinary = "/tmp/sesame"
	cfg.SESAMEDeployment = "/tmp/sesame-deployment"
	cfg.SESAMEIssuer = "https://mail.example.com"
	return cfg
}

func TestEnvironmentOverridesFileWithoutSecretFlags(t *testing.T) {
	t.Setenv("PORT", "8123")
	t.Setenv("SESAME_ISSUER", "https://environment.example.com")
	t.Setenv("INBOUND_WEBHOOK_SECRET", "environment-hook-secret")
	t.Setenv("FYLO_ROOT", "/tmp/caduceus-root")
	t.Setenv("FYLO_BINARY", "/tmp/caduceus-fylo")
	t.Setenv("SESAME_BINARY", "/tmp/caduceus-sesame")
	t.Setenv("SESAME_DEPLOYMENT", "/tmp/caduceus-sesame-deployment")
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"PORT":"7000"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8123" || cfg.SESAMEIssuer != "https://environment.example.com" {
		t.Fatalf("environment override not applied: %+v", cfg)
	}
}

func TestValidateRuntimeBoundaries(t *testing.T) {
	valid := Defaults()
	valid.FYLOBinary = "/tmp/fylo"
	valid.FYLORoot = "/tmp/root"
	valid.InboundWebhookSecret = "fedcba9876543210"
	valid.SESAMEBinary = "/tmp/sesame"
	valid.SESAMEDeployment = "/tmp/sesame-deployment"
	valid.SESAMEIssuer = "https://mail.example.com"
	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"empty host", func(cfg *Config) { cfg.Host = "" }, "HOST"},
		{"bad port", func(cfg *Config) { cfg.Port = "0" }, "PORT"},
		{"bad bridge port", func(cfg *Config) { cfg.FYLOBridgeAddress = "127.0.0.1:http" }, "FYLO_BRIDGE_ADDRESS"},
		{"noncanonical bridge loopback", func(cfg *Config) { cfg.FYLOBridgeAddress = "127.0.0.2:9876" }, "FYLO_BRIDGE_ADDRESS"},
		{"same listeners", func(cfg *Config) { cfg.MetricsPort = cfg.Port }, "different"},
		{"missing static", func(cfg *Config) { cfg.StaticDir = "" }, "STATIC_DIR"},
		{"missing SESAME binary", func(cfg *Config) { cfg.SESAMEBinary = "" }, "SESAME_BINARY"},
		{"missing SESAME deployment", func(cfg *Config) { cfg.SESAMEDeployment = "" }, "SESAME_DEPLOYMENT"},
		{"short webhook secret", func(cfg *Config) { cfg.InboundWebhookSecret = "short" }, "INBOUND_WEBHOOK_SECRET"},
		{"bad session lifetime", func(cfg *Config) { cfg.SESAMESessionLifetime = "10s" }, "SESAME_SESSION_LIFETIME"},
		{"bad duration", func(cfg *Config) { cfg.ShutdownTimeout = "forever" }, "SHUTDOWN_TIMEOUT"},
		{"bad external account timeout", func(cfg *Config) { cfg.ExternalAccountConnectionTimeout = "500ms" }, "EXTERNAL_ACCOUNT_CONNECTION_TIMEOUT"},
		{"short duration", func(cfg *Config) { cfg.ShutdownTimeout = "1ms" }, "between"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.change(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParsePortAndAddresses(t *testing.T) {
	if port, err := ParsePort("PORT", "65535"); err != nil || port != 65535 {
		t.Fatalf("ParsePort = %d, %v", port, err)
	}
	cfg := Defaults()
	if cfg.HTTPAddress() != "127.0.0.1:8080" || cfg.MetricsAddress() != "127.0.0.1:9090" {
		t.Fatalf("unexpected addresses: %s %s", cfg.HTTPAddress(), cfg.MetricsAddress())
	}
	if cfg.ShutdownDuration().String() != "10s" {
		t.Fatalf("shutdown duration = %s", cfg.ShutdownDuration())
	}
	if cfg.SnoozeWorker != "on" || cfg.SnoozeIntervalMS != "30000" || cfg.SnoozeTimeoutMS != "60000" {
		t.Fatalf("unexpected snooze worker defaults: %q %q %q", cfg.SnoozeWorker, cfg.SnoozeIntervalMS, cfg.SnoozeTimeoutMS)
	}
	if cfg.ReminderWorker != "on" || cfg.ReminderIntervalMS != "30000" || cfg.ReminderTimeoutMS != "60000" {
		t.Fatalf("unexpected reminder worker defaults: %q %q %q", cfg.ReminderWorker, cfg.ReminderIntervalMS, cfg.ReminderTimeoutMS)
	}
	cfg.FYLOBridgeAddress = "[::1]:9876"
	cfg.FYLOBinary = "/tmp/fylo"
	cfg.FYLORoot = "/tmp/root"
	cfg.InboundWebhookSecret = "fedcba9876543210"
	cfg.SESAMEBinary = "/tmp/sesame"
	cfg.SESAMEDeployment = "/tmp/sesame-deployment"
	cfg.SESAMEIssuer = "https://mail.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("IPv6 bridge address rejected: %v", err)
	}
}

func TestValidateKnownMigrationValues(t *testing.T) {
	valid := Defaults()
	valid.FYLOBinary = "/tmp/fylo"
	valid.FYLORoot = "/tmp/root"
	valid.InboundWebhookSecret = "fedcba9876543210"
	valid.SESAMEBinary = "/tmp/sesame"
	valid.SESAMEDeployment = "/tmp/sesame-deployment"
	valid.SESAMEIssuer = "https://mail.example.com"
	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"boolean", func(cfg *Config) { cfg.MailServerEnabled = "yes" }, "MAIL_SERVER_ENABLED"},
		{"mail port", func(cfg *Config) { cfg.SMTPInboundPort = "70000" }, "SMTP_INBOUND_PORT"},
		{"positive integer", func(cfg *Config) { cfg.SMTPMaxSize = "0" }, "SMTP_MAX_SIZE"},
		{"snooze interval", func(cfg *Config) { cfg.SnoozeIntervalMS = "0" }, "SNOOZE_INTERVAL_MS"},
		{"snooze worker", func(cfg *Config) { cfg.SnoozeWorker = "sometimes" }, "SNOOZE_WORKER"},
		{"reminder interval", func(cfg *Config) { cfg.ReminderIntervalMS = "0" }, "REMINDER_INTERVAL_MS"},
		{"reminder worker", func(cfg *Config) { cfg.ReminderWorker = "sometimes" }, "REMINDER_WORKER"},
		{"non-negative integer", func(cfg *Config) { cfg.MailStatusIntervalMS = "-1" }, "MAIL_STATUS_INTERVAL_MS"},
		{"enum", func(cfg *Config) { cfg.AuthPolicy = "sms" }, "CADUCEUS_AUTH_POLICY"},
		{"SMTP TLS policy", func(cfg *Config) { cfg.SMTPTLSPolicy = "downgrade" }, "SMTP_TLS_POLICY"},
		{"URL", func(cfg *Config) { cfg.PublicURL = "/relative" }, "PUBLIC_URL"},
		{"email", func(cfg *Config) { cfg.SMTPFrom = "not-an-email" }, "SMTP_FROM"},
		{"score", func(cfg *Config) { cfg.SpamScoreThreshold = "-1" }, "SPAM_SCORE_THRESHOLD"},
		{"prod console", func(cfg *Config) { cfg.NodeEnv = "production"; cfg.SMTPAdapter = "console" }, "forbidden"},
		{"prod tests", func(cfg *Config) { cfg.NodeEnv = "production"; cfg.EnableTestRoutes = "true" }, "forbidden"},
		{"prod ACME staging", func(cfg *Config) { cfg.NodeEnv = "production"; cfg.MailACMEStaging = "true" }, "forbidden"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.change(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate error = %v, want %q", err, test.want)
			}
		})
	}

	accepted := valid
	accepted.MailServerEnabled = "true"
	accepted.SMTPInboundPort = "25"
	accepted.MailStatusIntervalMS = "0"
	accepted.AuthPolicy = "passkey-first"
	accepted.PublicURL = "https://mail.example.com"
	accepted.SMTPFrom = "postmaster@example.com"
	accepted.SpamScoreThreshold = "5.5"
	if err := accepted.Validate(); err != nil {
		t.Fatalf("valid migration values rejected: %v", err)
	}
}
