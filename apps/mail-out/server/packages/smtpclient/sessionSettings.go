package smtpclient

import "crypto/tls"

type sessionSettings struct {
	Dialer      Dialer
	Address     string
	Host        string
	HeloName    string
	Username    string
	Password    string
	RequireTls  bool
	ImplicitTls bool
	Tls         *tls.Config
}

func (sessionSettings) defaultHeloName() string { return "localhost" }

func (settings sessionSettings) heloName() string {
	if settings.HeloName == "" {
		return settings.defaultHeloName()
	}
	return settings.HeloName
}

func (settings sessionSettings) dialer() Dialer {
	if settings.Dialer == nil {
		return DirectDialer{}
	}
	return settings.Dialer
}

func (settings sessionSettings) tlsConfig() *tls.Config {
	config := settings.Tls.Clone()
	config.ServerName = settings.serverName(config.ServerName)
	return config
}

func (settings sessionSettings) serverName(configured string) string {
	if configured == "" {
		return settings.Host
	}
	return configured
}
