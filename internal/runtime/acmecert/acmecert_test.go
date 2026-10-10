package acmecert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCA is a minimal RFC 8555 server: it trusts every JWS, validates the
// HTTP-01 answer over the network, and signs the CSR with a throwaway CA.
type fakeCA struct {
	t         *testing.T
	server    *httptest.Server
	httpPort  int
	mu        sync.Mutex
	token     string
	validated bool
	accounts  int
	caKey     *ecdsa.PrivateKey
	caCert    *x509.Certificate
	issued    []byte
}

func newFakeCA(t *testing.T, httpPort int) *fakeCA {
	ca := &fakeCA{t: t, httpPort: httpPort, token: "tok-123"}
	ca.caKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fake CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, &ca.caKey.PublicKey, ca.caKey)
	ca.caCert, _ = x509.ParseCertificate(der)
	ca.server = httptest.NewServer(http.HandlerFunc(ca.handle))
	t.Cleanup(ca.server.Close)
	return ca
}

func (ca *fakeCA) payload(request *http.Request) []byte {
	var jws struct {
		Payload string `json:"payload"`
	}
	body, _ := io.ReadAll(request.Body)
	_ = json.Unmarshal(body, &jws)
	decoded, _ := base64.RawURLEncoding.DecodeString(jws.Payload)
	return decoded
}

func (ca *fakeCA) handle(writer http.ResponseWriter, request *http.Request) {
	base := ca.server.URL
	writer.Header().Set("Replay-Nonce", strconv.FormatInt(time.Now().UnixNano(), 36))
	writeJSON := func(status int, value any) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(value)
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	switch request.URL.Path {
	case "/directory":
		writeJSON(200, map[string]any{"newNonce": base + "/nonce", "newAccount": base + "/account", "newOrder": base + "/order"})
	case "/nonce":
		writer.WriteHeader(200)
	case "/account":
		ca.accounts++
		writer.Header().Set("Location", base+"/account/1")
		writeJSON(201, map[string]any{"status": "valid", "contact": []string{"mailto:ops@mx.test"}})
	case "/order":
		writer.Header().Set("Location", base+"/order/1")
		writeJSON(201, ca.order())
	case "/order/1":
		writeJSON(200, ca.order())
	case "/authz/1":
		status := "pending"
		if ca.validated {
			status = "valid"
		}
		writeJSON(200, map[string]any{
			"status": status, "identifier": map[string]string{"type": "dns", "value": "mx.test"},
			"challenges": []map[string]any{
				{"type": "dns-01", "url": base + "/chall/dns", "token": "dns", "status": "pending"},
				{"type": "http-01", "url": base + "/chall/1", "token": ca.token, "status": status},
			},
		})
	case "/chall/1":
		ca.mu.Unlock()
		response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/.well-known/acme-challenge/%s", ca.httpPort, ca.token))
		ca.mu.Lock()
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			ca.validated = strings.HasPrefix(string(body), ca.token+".")
		}
		writeJSON(200, map[string]any{"type": "http-01", "url": base + "/chall/1", "token": ca.token, "status": "processing"})
	case "/finalize":
		var finalize struct {
			CSR string `json:"csr"`
		}
		_ = json.Unmarshal(ca.payload(request), &finalize)
		der, _ := base64.RawURLEncoding.DecodeString(finalize.CSR)
		csr, err := x509.ParseCertificateRequest(der)
		if err != nil {
			writeJSON(400, map[string]any{"type": "urn:ietf:params:acme:error:badCSR", "detail": err.Error()})
			return
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: csr.Subject, DNSNames: csr.DNSNames,
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour)}
		ca.issued, _ = x509.CreateCertificate(rand.Reader, leaf, ca.caCert, csr.PublicKey, ca.caKey)
		writeJSON(200, ca.order())
	case "/cert":
		writer.Header().Set("Content-Type", "application/pem-certificate-chain")
		_ = pem.Encode(writer, &pem.Block{Type: "CERTIFICATE", Bytes: ca.issued})
		_ = pem.Encode(writer, &pem.Block{Type: "CERTIFICATE", Bytes: ca.caCert.Raw})
	default:
		http.NotFound(writer, request)
	}
}

func (ca *fakeCA) order() map[string]any {
	base := ca.server.URL
	order := map[string]any{
		"status": "pending", "identifiers": []map[string]string{{"type": "dns", "value": "mx.test"}},
		"authorizations": []string{base + "/authz/1"}, "finalize": base + "/finalize",
	}
	if ca.validated {
		order["status"] = "ready"
	}
	if ca.issued != nil {
		order["status"], order["certificate"] = "valid", base+"/cert"
	}
	return order
}

func freePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestObtainsAndKeepsCertificate(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	ca := newFakeCA(t, port)
	var stdout strings.Builder
	service, err := New(Options{
		Hostname: "mx.test", Email: "ops@mx.test", HTTPPort: port, DirectoryURL: ca.server.URL + "/directory",
		CertPath: filepath.Join(dir, "tls", "mx.test.cert.pem"), KeyPath: filepath.Join(dir, "tls", "mx.test.key.pem"),
		AccountPath: filepath.Join(dir, "tls", "acme-account.pem"), SkipChallengeVerification: true, Stdout: &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := service.EnsureCertificate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Obtained || stdout.String() != "[acme] obtained certificate for mx.test (production)\n" {
		t.Fatalf("result = %+v stdout = %q", result, stdout.String())
	}
	if remaining := CertRemaining(filepath.Join(dir, "tls", "mx.test.cert.pem"), time.Now()); remaining < 80*24*time.Hour {
		t.Fatalf("remaining = %v", remaining)
	}
	keyPEM, _ := os.ReadFile(filepath.Join(dir, "tls", "mx.test.key.pem"))
	if !strings.HasPrefix(string(keyPEM), "-----BEGIN PRIVATE KEY-----") {
		t.Fatalf("key = %q", keyPEM[:40])
	}
	info, _ := os.Stat(filepath.Join(dir, "tls", "acme-account.pem"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("account key mode = %v", info.Mode())
	}
	// A valid certificate is kept.
	again, err := service.EnsureCertificate(ctx)
	if err != nil || again.Obtained || again.Reason != "cert still valid" {
		t.Fatalf("again = %+v %v", again, err)
	}
}

func TestReadsAcmeClientAccountKeys(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(key)
	for _, block := range []*pem.Block{
		{Type: "PRIVATE KEY", Bytes: pkcs8},
		{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)},
	} {
		if _, err := ParsePrivateKey(pem.EncodeToMemory(block)); err != nil {
			t.Fatalf("%s: %v", block.Type, err)
		}
	}
	if _, err := New(Options{Hostname: "mx.test"}); err == nil || err.Error() != "MAIL_HOSTNAME and MAIL_ACME_EMAIL are required for ACME" {
		t.Fatalf("err = %v", err)
	}
}

func TestChallengeServerAnswersLastSegment(t *testing.T) {
	store := &challengeStore{tokens: map[string]string{"abc": "abc.thumb"}}
	recorder := httptest.NewRecorder()
	store.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/abc", nil))
	if recorder.Code != 200 || recorder.Body.String() != "abc.thumb" || recorder.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
	missing := httptest.NewRecorder()
	store.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/x/nope", nil))
	if missing.Code != 404 {
		t.Fatalf("missing = %d", missing.Code)
	}
}
