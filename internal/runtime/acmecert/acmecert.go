// Package acmecert keeps the mail TLS certificate current with ACME
// (Let's Encrypt) HTTP-01, renewing it when fewer than 30 days remain.
//
// It ports background/AcmeService.js, which drove acme-client's `auto` flow.
// Key material stays compatible with what acme-client wrote: RSA 2048 keys
// as PKCS#8 PEM for the account and the certificate, and the certificate
// chain as PEM. An existing account key in PKCS#1, PKCS#8 or SEC 1 form is
// read as is.
package acmecert

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
)

// Let's Encrypt directories, as acme-client.directory.letsencrypt.
const (
	ProductionDirectory = "https://acme-v02.api.letsencrypt.org/directory"
	StagingDirectory    = "https://acme-staging-v02.api.letsencrypt.org/directory"

	renewBefore = 30 * 24 * time.Hour
	daily       = 24 * time.Hour
)

// Options configures the service.
type Options struct {
	Hostname    string
	Email       string
	CertPath    string
	KeyPath     string
	AccountPath string
	HTTPPort    int
	Staging     bool
	// DirectoryURL overrides the Let's Encrypt directory (tests).
	DirectoryURL string
	// SkipChallengeVerification skips the self-check of the HTTP-01 answer
	// before telling the CA to validate.
	SkipChallengeVerification bool
	Stdout                    io.Writer
	Stderr                    io.Writer
	Now                       func() time.Time
}

// Service obtains and renews one certificate.
type Service struct {
	options Options
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
}

// New validates options.
func New(options Options) (*Service, error) {
	if options.Hostname == "" || options.Email == "" {
		return nil, errors.New("MAIL_HOSTNAME and MAIL_ACME_EMAIL are required for ACME")
	}
	if options.HTTPPort == 0 {
		options.HTTPPort = 80
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{options: options}, nil
}

// Result reports what EnsureCertificate did.
type Result struct {
	Obtained bool
	Reason   string
}

// EnsureCertificate obtains or renews the certificate when it is missing or
// inside the renewal window.
func (service *Service) EnsureCertificate(ctx context.Context) (Result, error) {
	options := service.options
	if CertRemaining(options.CertPath, options.Now()) > renewBefore {
		return Result{Reason: "cert still valid"}, nil
	}
	accountKey, err := loadOrCreateAccountKey(options.AccountPath)
	if err != nil {
		return Result{}, err
	}
	directory := options.DirectoryURL
	if directory == "" {
		directory = ProductionDirectory
		if options.Staging {
			directory = StagingDirectory
		}
	}
	client := &acme.Client{Key: accountKey, DirectoryURL: directory, UserAgent: "caduceus"}

	certificateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return Result{}, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: options.Hostname},
		DNSNames: []string{options.Hostname},
	}, certificateKey)
	if err != nil {
		return Result{}, err
	}

	challenges := &challengeStore{tokens: map[string]string{}}
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(options.HTTPPort))
	if err != nil {
		return Result{}, err
	}
	server := &http.Server{Handler: challenges, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	chain, err := service.issue(ctx, client, csr, challenges)
	if err != nil {
		return Result{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificateKey)
	if err != nil {
		return Result{}, err
	}
	if err := WriteAtomic(options.KeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})); err != nil {
		return Result{}, err
	}
	var certificatePEM []byte
	for _, der := range chain {
		certificatePEM = append(certificatePEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	if err := WriteAtomic(options.CertPath, certificatePEM); err != nil {
		return Result{}, err
	}
	environment := "production"
	if options.Staging {
		environment = "staging"
	}
	fmt.Fprintf(options.Stdout, "[acme] obtained certificate for %s (%s)\n", options.Hostname, environment)
	return Result{Obtained: true}, nil
}

// issue runs acme-client's auto flow: account, order, HTTP-01 per
// authorization, finalize, download.
func (service *Service) issue(ctx context.Context, client *acme.Client, csr []byte, challenges *challengeStore) ([][]byte, error) {
	options := service.options
	account := &acme.Account{Contact: []string{"mailto:" + options.Email}}
	if _, err := client.Register(ctx, account, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return nil, err
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(options.Hostname))
	if err != nil {
		return nil, err
	}
	for _, url := range order.AuthzURLs {
		authorization, err := client.GetAuthorization(ctx, url)
		if err != nil {
			return nil, err
		}
		if authorization.Status == acme.StatusValid {
			continue
		}
		if err := service.satisfy(ctx, client, authorization, challenges); err != nil {
			return nil, err
		}
	}
	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return nil, err
	}
	chain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	return chain, err
}

func (service *Service) satisfy(ctx context.Context, client *acme.Client, authorization *acme.Authorization, challenges *challengeStore) error {
	var challenge *acme.Challenge
	for _, candidate := range authorization.Challenges {
		if candidate.Type == "http-01" {
			challenge = candidate
			break
		}
	}
	if challenge == nil && len(authorization.Challenges) > 0 {
		challenge = authorization.Challenges[0]
	}
	if challenge == nil {
		return fmt.Errorf("Unable to select challenge for %s, no challenge found", authorization.Identifier.Value)
	}
	if challenge.Type != "http-01" {
		return fmt.Errorf("Unsupported challenge type %s for %s", challenge.Type, authorization.Identifier.Value)
	}
	keyAuthorization, err := client.HTTP01ChallengeResponse(challenge.Token)
	if err != nil {
		return err
	}
	challenges.set(challenge.Token, keyAuthorization)
	defer challenges.remove(challenge.Token)
	if !service.options.SkipChallengeVerification {
		if err := verifyHTTP01(ctx, authorization.Identifier.Value, challenge.Token, keyAuthorization); err != nil {
			return err
		}
	}
	if _, err := client.Accept(ctx, challenge); err != nil {
		return err
	}
	_, err = client.WaitAuthorization(ctx, authorization.URI)
	return err
}

// verifyHTTP01 checks that the challenge answer is reachable before asking
// the CA to validate it, as acme-client's verifyChallenge did.
func verifyHTTP01(ctx context.Context, host, token, keyAuthorization string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/.well-known/acme-challenge/"+token, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if strings.TrimSpace(string(body)) != keyAuthorization {
		return fmt.Errorf("Authorization not found in HTTP response from %s", host)
	}
	return nil
}

// StartRenewal provisions the certificate now and re-checks daily; a failed
// daily check is logged, not fatal.
func (service *Service) StartRenewal(ctx context.Context) error {
	service.mu.Lock()
	if service.stop != nil {
		service.mu.Unlock()
		return nil
	}
	service.mu.Unlock()
	if _, err := service.EnsureCertificate(ctx); err != nil {
		return err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.stop, service.done = make(chan struct{}), make(chan struct{})
	stop, done := service.stop, service.done
	go func() {
		defer close(done)
		ticker := time.NewTicker(daily)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if _, err := service.EnsureCertificate(context.Background()); err != nil {
					fmt.Fprintf(service.options.Stderr, "[acme] renewal check failed: %v\n", err)
				}
			}
		}
	}()
	return nil
}

// Stop ends future renewal checks. It is safe to call more than once.
func (service *Service) Stop() {
	service.mu.Lock()
	stop, done := service.stop, service.done
	service.stop, service.done = nil, nil
	service.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

// CertRemaining answers how long the certificate at path stays valid; a
// missing or unreadable certificate is already expired.
func CertRemaining(path string, now time.Time) time.Duration {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1 << 63
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return -1 << 63
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return -1 << 63
	}
	return certificate.NotAfter.Sub(now)
}

// WriteAtomic writes a file through a temporary name and a rename, creating
// parent directories, with mode 0600.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	temporary := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func loadOrCreateAccountKey(path string) (crypto.Signer, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return ParsePrivateKey(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := WriteAtomic(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err != nil {
		return nil, err
	}
	return key, nil
}

// ParsePrivateKey reads a PEM private key in PKCS#8, PKCS#1 or SEC 1 form.
func ParsePrivateKey(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("ACME account key is not PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			return signer, nil
		}
		return nil, errors.New("ACME account key cannot sign")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("ACME account key is not a supported private key")
}

type challengeStore struct {
	mu     sync.Mutex
	tokens map[string]string
}

func (store *challengeStore) set(token, value string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.tokens[token] = value
}

func (store *challengeStore) remove(token string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.tokens, token)
}

// ServeHTTP answers the key authorization for the last path segment, as the
// JavaScript challenge server did.
func (store *challengeStore) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	token := path[strings.LastIndex(path, "/")+1:]
	store.mu.Lock()
	value, ok := store.tokens[token]
	store.mu.Unlock()
	if !ok || token == "" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(writer, value)
}
