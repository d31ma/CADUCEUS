package smtpclient

import (
	"context"
	"crypto/tls"
	"net"
)

type RelayDelivery struct {
	Address     string
	Dialer      Dialer
	HeloName    string
	Username    string
	Password    string
	RequireTls  bool
	ImplicitTls bool
	Tls         *tls.Config
}

func (relay RelayDelivery) Deliver(ctx context.Context, envelope Envelope, message []byte) (Reply, error) {
	return newSession(relay.settings(), envelope, message).run(ctx)
}

func (relay RelayDelivery) settings() sessionSettings {
	host, _, _ := net.SplitHostPort(relay.Address)
	return sessionSettings{
		Dialer:      relay.Dialer,
		Address:     relay.Address,
		Host:        host,
		HeloName:    relay.HeloName,
		Username:    relay.Username,
		Password:    relay.Password,
		RequireTls:  relay.RequireTls,
		ImplicitTls: relay.ImplicitTls,
		Tls:         relay.tlsConfig(),
	}
}

func (relay RelayDelivery) tlsConfig() *tls.Config {
	if relay.Tls == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return relay.Tls
}
