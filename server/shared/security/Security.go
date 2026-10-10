import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Security holds the input checks every route shares.
type Security struct{}

// NormalizeEmailAddress answers the lower-case address, or "" when it is not
// one.
func (Security) NormalizeEmailAddress(email string) string {
	value := strings.ToLower(strings.TrimSpace(email))
	if !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(value) {
		return ""
	}
	return value
}

// NormalizeDomain answers the lower-case domain without trailing dots, or ""
// when it is not a domain name.
func (Security) NormalizeDomain(domain string) string {
	value := strings.TrimRight(strings.ToLower(strings.TrimSpace(domain)), ".")
	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*\.[a-z]{2,}$`).MatchString(value) {
		return ""
	}
	return value
}

// HasDomainClaim reports whether domain is one of the caller's domains.
func (Security) HasDomainClaim(userDomains []string, domain string) bool {
	for _, candidate := range userDomains {
		if domain != "" && strings.EqualFold(candidate, domain) {
			return true
		}
	}
	return false
}

// SafeWebhookUrl refuses webhook URLs that would reach this server's loopback
// or an RFC 1918 network, and answers "" when the URL is safe.
func (security Security) SafeWebhookUrl(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "Invalid URL"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "Unsafe protocol"
	}
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return "Webhook URL must not be localhost"
	}
	if net.ParseIP(host) != nil && security.privateIPv4(host) {
		return "Webhook URL must not be a private address"
	}
	return ""
}

func (Security) privateIPv4(ip string) bool {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
		return true
	}
	parts := strings.Split(ip, ".")
	if !strings.HasPrefix(ip, "172.") || len(parts) < 2 {
		return false
	}
	second, _ := strconv.Atoi(parts[1])
	return second >= 16 && second <= 31
}

// TestRoutesEnabled reports whether the /test/* routes answer.
func (Security) TestRoutesEnabled() bool {
	return Settings{}.Value("CADUCEUS_ENABLE_TEST_ROUTES") == "true"
}
