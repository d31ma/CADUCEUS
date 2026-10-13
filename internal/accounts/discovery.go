package accounts

import (
	"context"
	"encoding/xml"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type DiscoverySource string

const (
	DiscoveryMX         DiscoverySource = "mx"
	DiscoveryAutoconfig DiscoverySource = "autoconfig"
	DiscoveryPreset     DiscoverySource = "preset"
	DiscoveryFallback   DiscoverySource = "fallback"
)

type MXResolver interface {
	LookupMX(context.Context, string) ([]string, error)
}

type AutoconfigFetcher interface {
	Fetch(context.Context, string) ([]byte, error)
}

type DiscoveryResult struct {
	ChoiceID   string             `json:"choiceId"`
	Source     DiscoverySource    `json:"source"`
	Connection ConnectionSettings `json:"connection"`
}

type Discovery struct {
	mx      MXResolver
	fetcher AutoconfigFetcher
}

func NewDiscovery(mx MXResolver, fetcher AutoconfigFetcher) *Discovery {
	return &Discovery{mx: mx, fetcher: fetcher}
}

func (service *Discovery) Discover(ctx context.Context, address string) (DiscoveryResult, error) {
	normalized, err := normalizeAddress(address)
	if err != nil {
		return DiscoveryResult{}, err
	}
	parts := strings.Split(normalized, "@")
	domain := parts[1]
	if choice, found := service.fromMX(ctx, domain); found {
		return resultForChoice(choice, DiscoveryMX), nil
	}
	if connection, found := service.fromAutoconfig(ctx, normalized, domain); found {
		return DiscoveryResult{ChoiceID: "imap", Source: DiscoveryAutoconfig, Connection: connection}, nil
	}
	if choice, found := choiceForDomain(domain); found {
		return resultForChoice(choice, DiscoveryPreset), nil
	}
	return DiscoveryResult{
		ChoiceID: "imap", Source: DiscoveryFallback,
		Connection: ConnectionSettings{
			IMAP: &Endpoint{Host: "imap." + domain, Port: 993, Security: SecurityTLS, Username: normalized},
			SMTP: &Endpoint{Host: "smtp." + domain, Port: 587, Security: SecurityStartTLS, Username: normalized},
		},
	}, nil
}

func (service *Discovery) fromMX(ctx context.Context, domain string) (ProviderChoice, bool) {
	if service.mx == nil {
		return ProviderChoice{}, false
	}
	hosts, err := service.mx.LookupMX(ctx, domain)
	if err != nil {
		return ProviderChoice{}, false
	}
	sort.Strings(hosts)
	for _, host := range hosts {
		if id := choiceIDForMX(strings.ToLower(strings.TrimSuffix(host, "."))); id != "" {
			return choiceByID(id)
		}
	}
	return ProviderChoice{}, false
}

func (service *Discovery) fromAutoconfig(ctx context.Context, address, domain string) (ConnectionSettings, bool) {
	if service.fetcher == nil {
		return ConnectionSettings{}, false
	}
	query := "?emailaddress=" + url.QueryEscape(address)
	urls := []string{
		"https://autoconfig." + domain + "/mail/config-v1.1.xml" + query,
		"https://" + domain + "/.well-known/autoconfig/mail/config-v1.1.xml" + query,
	}
	for _, target := range urls {
		body, err := service.fetcher.Fetch(ctx, target)
		if err != nil {
			continue
		}
		connection, err := parseAutoconfig(body, address)
		if err == nil {
			return connection, true
		}
	}
	return ConnectionSettings{}, false
}

func resultForChoice(choice ProviderChoice, source DiscoverySource) DiscoveryResult {
	result := DiscoveryResult{ChoiceID: choice.ID, Source: source}
	if choice.Preset != nil {
		result.Connection = cloneConnection(*choice.Preset)
	}
	return result
}

func cloneConnection(connection ConnectionSettings) ConnectionSettings {
	return ConnectionSettings{
		IMAP: cloneEndpoint(connection.IMAP), SMTP: cloneEndpoint(connection.SMTP), ContainerFolder: connection.ContainerFolder,
	}
}

func choiceByID(id string) (ProviderChoice, bool) {
	for _, choice := range ProviderCatalog() {
		if choice.ID == id {
			return choice, true
		}
	}
	return ProviderChoice{}, false
}

func choiceForDomain(domain string) (ProviderChoice, bool) {
	domains := map[string]string{
		"gmail.com": "gmail", "googlemail.com": "gmail",
		"outlook.com": "outlook", "hotmail.com": "outlook", "live.com": "outlook", "msn.com": "outlook",
		"icloud.com": "icloud", "me.com": "icloud", "mac.com": "icloud",
		"yahoo.com": "yahoo", "ymail.com": "yahoo", "rocketmail.com": "yahoo",
		"fastmail.com": "fastmail", "fastmail.fm": "fastmail",
		"gmx.com": "gmx", "gmx.net": "gmx", "yandex.com": "yandex", "yandex.ru": "yandex",
	}
	return choiceByID(domains[domain])
}

func choiceIDForMX(host string) string {
	tests := []struct {
		fragment string
		id       string
	}{
		{"google.com", "google-workspace"}, {"googlemail.com", "google-workspace"},
		{"protection.outlook.com", "office365"}, {"yahoodns.net", "yahoo"},
		{"icloud.com", "icloud"}, {"fastmail.com", "fastmail"}, {"gmx.net", "gmx"}, {"yandex.net", "yandex"},
	}
	for _, test := range tests {
		if strings.HasSuffix(host, test.fragment) {
			return test.id
		}
	}
	return ""
}

type clientConfigXML struct {
	Provider struct {
		Incoming []serverXML `xml:"incomingServer"`
		Outgoing []serverXML `xml:"outgoingServer"`
	} `xml:"emailProvider"`
}

type serverXML struct {
	Type       string `xml:"type,attr"`
	Hostname   string `xml:"hostname"`
	Port       string `xml:"port"`
	SocketType string `xml:"socketType"`
	Username   string `xml:"username"`
}

func parseAutoconfig(body []byte, address string) (ConnectionSettings, error) {
	var config clientConfigXML
	if len(body) == 0 || len(body) > 1<<20 || xml.Unmarshal(body, &config) != nil {
		return ConnectionSettings{}, ErrInvalid
	}
	imap := selectServer(config.Provider.Incoming, "imap", address)
	smtp := selectServer(config.Provider.Outgoing, "smtp", address)
	if validateEndpoint(imap) != nil || validateEndpoint(smtp) != nil {
		return ConnectionSettings{}, ErrInvalid
	}
	return ConnectionSettings{IMAP: imap, SMTP: smtp}, nil
}

func selectServer(servers []serverXML, kind, address string) *Endpoint {
	for _, server := range servers {
		if strings.ToLower(server.Type) != kind {
			continue
		}
		port, err := strconv.ParseUint(strings.TrimSpace(server.Port), 10, 16)
		security, ok := socketSecurity(server.SocketType)
		if err != nil || !ok {
			return nil
		}
		return &Endpoint{
			Host: server.Hostname, Port: uint16(port), Security: security,
			Username: expandUsername(server.Username, address),
		}
	}
	return nil
}

func socketSecurity(value string) (SecurityMode, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ssl", "ssl/tls", "tls":
		return SecurityTLS, true
	case "starttls":
		return SecurityStartTLS, true
	case "plain", "none":
		return SecurityNone, true
	default:
		return "", false
	}
}

func expandUsername(template, address string) string {
	parts := strings.Split(address, "@")
	replacer := strings.NewReplacer(
		"%EMAILADDRESS%", address, "%EMAILLOCALPART%", parts[0], "%EMAILDOMAIN%", parts[1],
	)
	return replacer.Replace(strings.TrimSpace(template))
}
