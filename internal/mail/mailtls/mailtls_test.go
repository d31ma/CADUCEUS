package mailtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pair(t *testing.T, name string) (string, string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func commonName(t *testing.T, certificate *tls.Certificate) string {
	t.Helper()
	parsed, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Subject.CommonName
}

func TestLoadAndReloadFromFiles(t *testing.T) {
	dir := t.TempDir()
	keyFile, certFile := filepath.Join(dir, "key.pem"), filepath.Join(dir, "cert.pem")
	key, cert := pair(t, "first")
	_ = os.WriteFile(keyFile, []byte(key), 0o600)
	_ = os.WriteFile(certFile, []byte(cert), 0o600)
	t.Setenv("MAIL_TLS_KEY", "")
	t.Setenv("MAIL_TLS_CERT", "")
	t.Setenv("MAIL_TLS_KEY_FILE", keyFile)
	t.Setenv("MAIL_TLS_CERT_FILE", certFile)
	material := Load()
	if material == nil || material.Key != key {
		t.Fatal("material not loaded from files")
	}
	config, err := Config(material)
	if err != nil {
		t.Fatal(err)
	}
	renewedKey, renewedCert := pair(t, "renewed")
	_ = os.WriteFile(keyFile, []byte(renewedKey), 0o600)
	_ = os.WriteFile(certFile, []byte(renewedCert), 0o600)
	withSNI, err := config.GetCertificate(&tls.ClientHelloInfo{ServerName: "mx.test"})
	if err != nil || commonName(t, withSNI) != "renewed" {
		t.Fatalf("SNI certificate = %v", err)
	}
	withoutSNI, _ := config.GetCertificate(&tls.ClientHelloInfo{})
	if commonName(t, withoutSNI) != "first" {
		t.Fatal("handshake without SNI did not use the startup certificate")
	}
	t.Setenv("MAIL_TLS_CERT_FILE", filepath.Join(dir, "missing.pem"))
	if Load() != nil {
		t.Fatal("half the material answered")
	}
}
