package protocol

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type certificatePair struct {
	certificatePath string
	keyPath         string
	pool            *x509.CertPool
}

func writeCertificate(t *testing.T, folder string, serial int64) certificatePair {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pair := certificatePair{certificatePath: filepath.Join(folder, "cert.pem"), keyPath: filepath.Join(folder, "key.pem"), pool: x509.NewCertPool()}
	os.WriteFile(pair.certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(pair.keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o600)
	parsed, _ := x509.ParseCertificate(der)
	pair.pool.AddCert(parsed)
	return pair
}

func TestStartTlsUpgradesAndRequiresANewGreeting(t *testing.T) {
	pair := writeCertificate(t, t.TempDir(), 1)
	server := newHarness(t, func(settings *Settings) {
		settings.Tls = NewCertificateFiles(pair.certificatePath, pair.keyPath).TlsSettings()
	})
	client, err := smtp.Dial(server.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if offered, _ := client.Extension("STARTTLS"); !offered {
		t.Fatal("STARTTLS is not offered")
	}
	if err := client.StartTLS(&tls.Config{ServerName: "localhost", RootCAs: pair.pool}); err != nil {
		t.Fatal(err)
	}
	if offered, _ := client.Extension("STARTTLS"); offered {
		t.Fatal("STARTTLS is offered again inside TLS")
	}
	client.Mail("dan@outside.test")
	client.Rcpt("ana@example.com")
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	writer.Write([]byte(hostedMessage()))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !server.delivery.delivered()[0].Tls {
		t.Fatal("the transaction was not marked as TLS")
	}
}

func TestStartTlsAfterTlsAndBeforeGreetingAreRefused(t *testing.T) {
	pair := writeCertificate(t, t.TempDir(), 1)
	server := newHarness(t, func(settings *Settings) {
		settings.Tls = NewCertificateFiles(pair.certificatePath, pair.keyPath).TlsSettings()
	})
	plain := dialRaw(t, server.address)
	plain.expect("STARTTLS", 503, "Send EHLO")
	client, err := smtp.Dial(server.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.StartTLS(&tls.Config{ServerName: "localhost", RootCAs: pair.pool})
	client.Hello("outside.test")
	client.Text.Cmd("STARTTLS")
	reply, text, _ := client.Text.ReadResponse(0)
	if reply != 503 || !strings.Contains(text, "already") {
		t.Fatalf("want 503 already, got %d %q", reply, text)
	}
}

func TestFailedHandshakeClosesTheSession(t *testing.T) {
	pair := writeCertificate(t, t.TempDir(), 1)
	server := newHarness(t, func(settings *Settings) {
		settings.Tls = NewCertificateFiles(pair.certificatePath, pair.keyPath).TlsSettings()
	})
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "STARTTLS")
	client.expect("STARTTLS", 220, "2.0.0")
	client.text.PrintfLine("this is not a TLS hello")
	if _, err := client.text.ReadLine(); err == nil {
		t.Fatal("the session stayed open after a failed handshake")
	}
}

func TestCertificateFilesReloadAfterRenewal(t *testing.T) {
	folder := t.TempDir()
	writeCertificate(t, folder, 1)
	files := NewCertificateFiles(filepath.Join(folder, "cert.pem"), filepath.Join(folder, "key.pem"))
	first, err := files.Certificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := files.Certificate(nil)
	writeCertificate(t, folder, 2)
	later := time.Now().Add(time.Minute)
	os.Chtimes(filepath.Join(folder, "cert.pem"), later, later)
	renewed, err := files.Certificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != again || first == renewed {
		t.Fatal("the cache did not follow the file modification time")
	}
}

func TestMissingCertificateFileIsAnError(t *testing.T) {
	folder := t.TempDir()
	if _, err := NewCertificateFiles(filepath.Join(folder, "cert.pem"), filepath.Join(folder, "key.pem")).Certificate(nil); err == nil {
		t.Fatal("want an error")
	}
}

func TestMissingKeyFileIsAnError(t *testing.T) {
	folder := t.TempDir()
	pair := writeCertificate(t, folder, 1)
	if _, err := NewCertificateFiles(pair.certificatePath, filepath.Join(folder, "missing.pem")).Certificate(nil); err == nil {
		t.Fatal("want an error")
	}
}

func TestBrokenCertificateIsAnError(t *testing.T) {
	folder := t.TempDir()
	pair := writeCertificate(t, folder, 1)
	os.WriteFile(pair.certificatePath, []byte("not a certificate"), 0o600)
	if _, err := NewCertificateFiles(pair.certificatePath, pair.keyPath).Certificate(nil); err == nil {
		t.Fatal("want an error")
	}
}
