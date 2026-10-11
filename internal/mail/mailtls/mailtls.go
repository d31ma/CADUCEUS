// Package mailtls loads the TLS material the SMTP servers present.
//
// It ports InboundSmtpServer.loadTls, SubmissionServer.loadTls and
// AcmeService.sniFromFiles: the key and certificate come from MAIL_TLS_KEY /
// MAIL_TLS_CERT (inline PEM) or MAIL_TLS_KEY_FILE / MAIL_TLS_CERT_FILE, and
// when they live in files every SNI handshake reads them again, so ACME
// renewals apply without a restart.
package mailtls

import (
	"crypto/tls"
	"os"

	"github.com/d31ma/caduceus/internal/runtime/settings"
)

// Material is the PEM key and certificate.
type Material struct {
	Key  string
	Cert string
}

func safeRead(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// Load reads the TLS material from the settings, or answers nil when either
// half is missing.
func Load() *Material {
	key := settings.Value("MAIL_TLS_KEY")
	if key == "" {
		if path := settings.Value("MAIL_TLS_KEY_FILE"); path != "" {
			key = safeRead(path)
		}
	}
	cert := settings.Value("MAIL_TLS_CERT")
	if cert == "" {
		if path := settings.Value("MAIL_TLS_CERT_FILE"); path != "" {
			cert = safeRead(path)
		}
	}
	if key == "" || cert == "" {
		return nil
	}
	return &Material{Key: key, Cert: cert}
}

// Config answers the server TLS configuration for material. When the key
// and certificate files are configured, a handshake that names a server
// (SNI) reads them afresh; one without SNI uses the material loaded at
// start, as Node's SNICallback did.
func Config(material *Material) (*tls.Config, error) {
	if material == nil {
		return nil, nil
	}
	initial, err := tls.X509KeyPair([]byte(material.Cert), []byte(material.Key))
	if err != nil {
		return nil, err
	}
	config := &tls.Config{Certificates: []tls.Certificate{initial}, MinVersion: tls.VersionTLS12}
	keyFile, certFile := settings.Value("MAIL_TLS_KEY_FILE"), settings.Value("MAIL_TLS_CERT_FILE")
	if keyFile != "" && certFile != "" {
		config.GetCertificate = FromFiles(keyFile, certFile, &initial)
	}
	return config, nil
}

// FromFiles answers a GetCertificate that reads the files on every SNI
// handshake. Rebuilding per handshake is cheap next to the handshake itself.
func FromFiles(keyFile, certFile string, fallback *tls.Certificate) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName == "" && fallback != nil {
			return fallback, nil
		}
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		return &certificate, nil
	}
}
