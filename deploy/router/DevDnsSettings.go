package main

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
)

type DevDnsSettings struct {
	Domains  []string
	MailHost string
	Spf      string
}

func (settings DevDnsSettings) On() bool {
	return len(settings.Domains) > 0
}

func (settings DevDnsSettings) Problem() error {
	return errors.Join(
		settings.whenOn(settings.mismatch(settings.MailHostName(), settings.MailHost, settings.hostPattern())),
		settings.whenOn(settings.mismatch(settings.SpfName(), settings.Spf, settings.termPattern())),
		settings.domainProblem(),
	)
}

func (settings DevDnsSettings) whenOn(problem error) error {
	if settings.On() {
		return problem
	}
	return nil
}

func (settings DevDnsSettings) mismatch(name string, value string, pattern string) error {
	if regexp.MustCompile(pattern).MatchString(value) {
		return nil
	}
	return InvalidSetting{Name: name, Reason: "set a value that matches " + pattern + " when " + settings.DomainsName() + " is set"}
}

func (settings DevDnsSettings) domainProblem() error {
	index := slices.IndexFunc(settings.Domains, settings.badDomain)
	if index < 0 {
		return nil
	}
	return InvalidSetting{Name: settings.DomainsName(), Reason: "bad domain name " + strconv.Quote(settings.Domains[index]) + "; use lowercase host names"}
}

func (settings DevDnsSettings) badDomain(domain string) bool {
	return !regexp.MustCompile(settings.hostPattern()).MatchString(domain)
}

func (DevDnsSettings) hostPattern() string {
	return `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`
}

func (DevDnsSettings) termPattern() string {
	return `^[^\s"\\]+$`
}

func (DevDnsSettings) DomainsName() string {
	return "MESAIL_ROUTER_DEV_DNS_DOMAINS"
}

func (DevDnsSettings) MailHostName() string {
	return "MESAIL_ROUTER_DEV_DNS_MAIL_HOST"
}

func (DevDnsSettings) SpfName() string {
	return "MESAIL_ROUTER_DEV_DNS_SPF"
}
