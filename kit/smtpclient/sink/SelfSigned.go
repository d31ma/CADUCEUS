package sink

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"time"
)

type SelfSigned struct {
	Server *tls.Config
	Client *tls.Config
}

func NewSelfSigned() (SelfSigned, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return SelfSigned{}, err
	}
	der, err := x509.CreateCertificate(rand.Reader, SelfSigned{}.template(), SelfSigned{}.template(), &key.PublicKey, key)
	if err != nil {
		return SelfSigned{}, err
	}
	return SelfSigned{}.configs(der, key)
}

func (SelfSigned) validity() time.Duration { return time.Hour }

func (self SelfSigned) template() *x509.Certificate {
	now := time.Now()
	return &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             now.Add(-self.validity()),
		NotAfter:              now.Add(self.validity()),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
}

func (SelfSigned) configs(der []byte, key *ecdsa.PrivateKey) (SelfSigned, error) {
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return SelfSigned{}, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificate)
	pair := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: certificate}
	return SelfSigned{
		Server: &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12},
		Client: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12},
	}, nil
}
