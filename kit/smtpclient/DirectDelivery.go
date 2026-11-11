package smtpclient

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
)

type DirectDelivery struct {
	Hosts    MxHosts
	Dialer   Dialer
	HeloName string
	Port     int
	Tls      *tls.Config
}

func (delivery DirectDelivery) Deliver(ctx context.Context, envelope Envelope, message []byte) (Reply, error) {
	domain, err := delivery.domain(envelope.Recipients)
	if err != nil {
		return failures{}.badEnvelope(), err
	}
	hosts, err := delivery.Hosts.ForDomain(ctx, domain)
	if err != nil {
		return failures{}.wrap(err, domain).Reply, err
	}
	return delivery.tryHosts(ctx, hosts, envelope, message)
}

func (DirectDelivery) defaultPort() int { return 25 }

func (delivery DirectDelivery) domain(recipients []string) (string, error) {
	domains := slices.Compact(slices.Sorted(slices.Values(delivery.domains(recipients))))
	if len(domains) != 1 {
		return "", failures{}.permanent("", failures{}.badEnvelope())
	}
	return domains[0], nil
}

func (delivery DirectDelivery) domains(recipients []string) []string {
	domains := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		domains = append(domains, strings.ToLower(recipient[strings.LastIndex(recipient, "@")+1:]))
	}
	return domains
}

func (delivery DirectDelivery) tryHosts(ctx context.Context, hosts []string, envelope Envelope, message []byte) (Reply, error) {
	reply, err := Reply{}, error(failures{}.temporary("", failures{}.noHosts()))
	for index := 0; index < len(hosts) && delivery.retryable(err); index++ {
		reply, err = newSession(delivery.settings(hosts[index]), envelope, message).run(ctx)
	}
	return reply, err
}

func (delivery DirectDelivery) retryable(err error) bool {
	var failure DeliveryError
	return errors.As(err, &failure) && !failure.Permanent
}

func (delivery DirectDelivery) settings(host string) sessionSettings {
	return sessionSettings{
		Dialer:   delivery.Dialer,
		Address:  net.JoinHostPort(host, strconv.Itoa(cmp.Or(delivery.Port, delivery.defaultPort()))),
		Host:     host,
		HeloName: delivery.HeloName,
		Tls:      delivery.tlsConfig(),
	}
}

func (delivery DirectDelivery) tlsConfig() *tls.Config {
	if delivery.Tls == nil {
		return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	}
	return delivery.Tls
}
