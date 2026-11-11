package startup

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mesail.dev/edge/smtp/adapters"
	"mesail.dev/edge/smtp/mailchecks"
	"mesail.dev/edge/smtp/reception"
)

func baseSettings(t *testing.T) ListenerSettings {
	return ListenerSettings{Address: "127.0.0.1:0", Hostname: "mx.mesail.test", MaxSize: 1024, MaxConnections: 2, DirectoryUrl: "http://directory.local", MailInUrl: "http://mail-in.local", RawStoreFolder: t.TempDir(), ServiceToken: "service-token", VerifyAuth: true}
}

func writePair(t *testing.T) (string, string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	keyDer, _ := x509.MarshalECPrivateKey(key)
	folder := t.TempDir()
	certificatePath, keyPath := filepath.Join(folder, "cert.pem"), filepath.Join(folder, "key.pem")
	os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o600)
	return certificatePath, keyPath
}

func TestWiringWithoutCertificateOffersNoTls(t *testing.T) {
	built, err := Wiring{}.Build(baseSettings(t), log.New(io.Discard, "", 0))
	if err != nil || built.Tls != nil || built.Hostname != "mx.mesail.test" || built.Directory == nil || built.Delivery == nil {
		t.Fatalf("unexpected settings %+v %v", built, err)
	}
}

func TestWiringWithCertificateOffersTls(t *testing.T) {
	settings := baseSettings(t)
	settings.CertificateFile, settings.KeyFile = writePair(t)
	built, err := Wiring{}.Build(settings, log.New(io.Discard, "", 0))
	if err != nil || built.Tls == nil {
		t.Fatalf("want TLS, got %+v %v", built.Tls, err)
	}
}

func TestWiringWithBrokenCertificateFailsAtStartup(t *testing.T) {
	settings := baseSettings(t)
	settings.CertificateFile = filepath.Join(t.TempDir(), "missing.pem")
	settings.KeyFile = settings.CertificateFile
	if _, err := (Wiring{}).Build(settings, log.New(io.Discard, "", 0)); err == nil {
		t.Fatal("want an error")
	}
}

func TestWiringPicksTheTokenSource(t *testing.T) {
	settings := baseSettings(t)
	static := Wiring{}.tokensFor(settings, nil)
	settings.ServiceToken = ""
	settings.TokenSettings = adapters.TokenSettings{Url: "http://keycloak.local/token", ClientId: "smtp-edge", SecretFile: "/run/secret"}
	credentials := Wiring{}.tokensFor(settings, nil)
	token, _ := static.Token(context.Background())
	if token != "service-token" {
		t.Fatalf("unexpected static token %q", token)
	}
	if _, ok := credentials.(*adapters.ClientCredentialTokens); !ok {
		t.Fatalf("unexpected token source %T", credentials)
	}
}

func TestWiringPicksTheAuthChecker(t *testing.T) {
	settings := baseSettings(t)
	checking := Wiring{}.checkerFor(settings)
	settings.VerifyAuth = false
	skipping := Wiring{}.checkerFor(settings)
	if _, ok := checking.(mailchecks.MailAuthChecker); !ok {
		t.Fatalf("unexpected checker %T", checking)
	}
	if _, ok := skipping.(reception.SkippedChecks); !ok {
		t.Fatalf("unexpected checker %T", skipping)
	}
}
