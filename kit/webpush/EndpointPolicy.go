package webpush

import (
	"cmp"
	"net/url"
	"slices"
	"strings"
)

type EndpointPolicy struct {
	Hosts []string
}

func (EndpointPolicy) DefaultHosts() []string {
	return []string{"fcm.googleapis.com", "updates.push.services.mozilla.com", "push.apple.com", "notify.windows.com"}
}

func (policy EndpointPolicy) Check(endpoint string) error {
	_, err := policy.address(endpoint)
	return err
}

func (policy EndpointPolicy) address(endpoint string) (*url.URL, error) {
	address, err := url.Parse(endpoint)
	if err != nil {
		return nil, RefusedEndpoint{Reason: "the endpoint is not a valid URL"}
	}
	return address, cmp.Or(policy.scheme(address), policy.userinfo(address), policy.port(address), policy.host(address))
}

func (policy EndpointPolicy) scheme(address *url.URL) error {
	return policy.refusal(address.Scheme == policy.secureScheme(), "the endpoint must use https")
}

func (policy EndpointPolicy) userinfo(address *url.URL) error {
	return policy.refusal(address.User == nil, "the endpoint must not contain a user name or password")
}

func (policy EndpointPolicy) port(address *url.URL) error {
	return policy.refusal(policy.portAllowed(address.Port()), "the endpoint must use port 443")
}

func (policy EndpointPolicy) host(address *url.URL) error {
	return policy.refusal(policy.hostAllowed(address.Hostname()), "the endpoint host is not a known push service")
}

func (policy EndpointPolicy) portAllowed(port string) bool {
	return port == "" || port == policy.securePort()
}

func (policy EndpointPolicy) hostAllowed(hostname string) bool {
	return slices.ContainsFunc(policy.Hosts, func(entry string) bool {
		return policy.matches(strings.ToLower(hostname), strings.ToLower(entry))
	})
}

func (EndpointPolicy) matches(hostname string, entry string) bool {
	return entry != "" && strings.HasSuffix("."+hostname, "."+entry)
}

func (EndpointPolicy) refusal(allowed bool, reason string) error {
	if !allowed {
		return RefusedEndpoint{Reason: reason}
	}
	return nil
}

func (EndpointPolicy) secureScheme() string {
	return "https"
}

func (EndpointPolicy) securePort() string {
	return "443"
}
