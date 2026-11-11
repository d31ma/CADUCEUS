package smtpclient

import (
	"cmp"
	"net/url"
	"os"
	"strings"
)

type ProxyDialers struct{}

func (proxies ProxyDialers) FromEnvironment() (Dialer, error) {
	return proxies.dialerFor(cmp.Or(os.Getenv("HTTPS_PROXY"), os.Getenv("https_proxy")))
}

func (proxies ProxyDialers) dialerFor(setting string) (Dialer, error) {
	if setting == "" {
		return DirectDialer{}, nil
	}
	return proxies.parse(setting)
}

func (proxies ProxyDialers) parse(setting string) (Dialer, error) {
	proxyUrl, err := url.Parse(proxies.withScheme(setting))
	if err != nil {
		return nil, err
	}
	return ProxyDialer{ProxyUrl: proxyUrl}, nil
}

func (ProxyDialers) withScheme(setting string) string {
	if strings.Contains(setting, "://") {
		return setting
	}
	return "http://" + setting
}
