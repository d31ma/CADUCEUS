package accounts

type AuthenticationMethod string
type ProviderGuidance string

const (
	AuthenticationOAuth    AuthenticationMethod = "oauth"
	AuthenticationPassword AuthenticationMethod = "password"

	GuidanceNone        ProviderGuidance = "none"
	GuidanceAppPassword ProviderGuidance = "app_password"
	GuidanceEnableIMAP  ProviderGuidance = "enable_imap"
)

type ProviderChoice struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Provider       Provider             `json:"provider"`
	Authentication AuthenticationMethod `json:"authentication"`
	Guidance       ProviderGuidance     `json:"guidance"`
	Preset         *ConnectionSettings  `json:"preset"`
	Configured     bool                 `json:"configured"`
}

// ProviderCatalog returns a stable onboarding order. Hosted brands backed by
// standards remain IMAP adapters with operator-visible presets; their domains
// never become CADUCEUS tenant domains.
func ProviderCatalog() []ProviderChoice {
	return []ProviderChoice{
		{ID: "gmail", Name: "Gmail", Provider: ProviderGmail, Authentication: AuthenticationOAuth, Guidance: GuidanceNone},
		{ID: "google-workspace", Name: "Google Workspace", Provider: ProviderGmail, Authentication: AuthenticationOAuth, Guidance: GuidanceNone},
		{ID: "office365", Name: "Microsoft 365", Provider: ProviderMicrosoft, Authentication: AuthenticationOAuth, Guidance: GuidanceNone},
		passwordChoice("yahoo", "Yahoo", GuidanceAppPassword, "imap.mail.yahoo.com", 993, SecurityTLS, "smtp.mail.yahoo.com", 465, SecurityTLS),
		{ID: "outlook", Name: "Outlook / Hotmail", Provider: ProviderMicrosoft, Authentication: AuthenticationOAuth, Guidance: GuidanceNone},
		passwordChoice("icloud", "Apple iCloud", GuidanceAppPassword, "imap.mail.me.com", 993, SecurityTLS, "smtp.mail.me.com", 587, SecurityStartTLS),
		passwordChoice("fastmail", "Fastmail", GuidanceAppPassword, "imap.fastmail.com", 993, SecurityTLS, "smtp.fastmail.com", 465, SecurityTLS),
		passwordChoice("gmx", "GMX", GuidanceEnableIMAP, "imap.gmx.com", 993, SecurityTLS, "mail.gmx.com", 465, SecurityTLS),
		passwordChoice("yandex", "Yandex", GuidanceEnableIMAP, "imap.yandex.com", 993, SecurityTLS, "smtp.yandex.com", 465, SecurityTLS),
		{ID: "imap", Name: "IMAP / SMTP", Provider: ProviderIMAP, Authentication: AuthenticationPassword, Guidance: GuidanceNone},
	}
}

func ProviderCatalogWithMicrosoftOAuth(configured bool) []ProviderChoice {
	return ProviderCatalogWithOAuth(configured, false)
}

func ProviderCatalogWithOAuth(microsoftConfigured, googleConfigured bool) []ProviderChoice {
	providers := ProviderCatalog()
	for index := range providers {
		providers[index].Configured = providers[index].Authentication == AuthenticationPassword ||
			providers[index].Provider == ProviderMicrosoft && microsoftConfigured || providers[index].Provider == ProviderGmail && googleConfigured
	}
	return providers
}

func passwordChoice(id, name string, guidance ProviderGuidance, imapHost string, imapPort uint16, imapSecurity SecurityMode, smtpHost string, smtpPort uint16, smtpSecurity SecurityMode) ProviderChoice {
	return ProviderChoice{
		ID: id, Name: name, Provider: ProviderIMAP, Authentication: AuthenticationPassword, Guidance: guidance,
		Configured: true,
		Preset: &ConnectionSettings{
			IMAP: &Endpoint{Host: imapHost, Port: imapPort, Security: imapSecurity},
			SMTP: &Endpoint{Host: smtpHost, Port: smtpPort, Security: smtpSecurity},
		},
	}
}

func SecurityForStandardPort(protocol string, port uint16) (SecurityMode, bool) {
	ports := map[string]map[uint16]SecurityMode{
		"imap": {143: SecurityNone, 993: SecurityTLS},
		"smtp": {25: SecurityNone, 465: SecurityTLS, 587: SecurityStartTLS},
	}
	security, found := ports[protocol][port]
	return security, found
}
