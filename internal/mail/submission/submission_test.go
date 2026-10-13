package submission

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/mail/mailtls"
	"github.com/d31ma/caduceus/internal/mail/smtpd"
)

func material(t *testing.T) *mailtls.Material {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	return &mailtls.Material{
		Cert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		Key:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
	}
}

func TestRequiresTLS(t *testing.T) {
	if _, err := Start(Options{Host: "127.0.0.1"}); err == nil || err.Error() != "SMTP submission requires TLS (MAIL_TLS_CERT/KEY)" {
		t.Fatalf("err = %v", err)
	}
}

func start(t *testing.T, options Options) *Server {
	t.Helper()
	options.Host = "127.0.0.1"
	options.Hostname = "smtp.test"
	options.MaxSize = 1 << 20
	options.SMTP = func(smtp *smtpd.Options) { smtp.DisableReverseLookup = true }
	server, err := Start(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})
	return server
}

func TestRefusesAuthenticationAfterSTARTTLS(t *testing.T) {
	server := start(t, Options{TLS: material(t)})
	client, err := smtp.Dial(server.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	err = client.Auth(smtp.PlainAuth("", "user@d.test", "secret", "127.0.0.1"))
	if err == nil || !strings.Contains(err.Error(), "535") || !strings.Contains(err.Error(), "application password") {
		t.Fatalf("auth err = %v", err)
	}
}

func TestImplicitTLSListener(t *testing.T) {
	server := start(t, Options{TLS: material(t), Secure: true})
	conn, err := tls.Dial("tcp", server.Addr().String(), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	greeting := make([]byte, 128)
	n, _ := conn.Read(greeting)
	if got := string(greeting[:n]); got != "220 smtp.test ESMTP smtp.test ESMTP CADUCEUS submission\r\n" {
		t.Fatalf("greeting = %q", got)
	}
}

func TestOnDataPushesThroughSend(t *testing.T) {
	var gotBody, gotAuth string
	status := http.StatusOK
	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		gotBody, gotAuth = string(body), request.Header.Get("Authorization")
		writer.WriteHeader(status)
	}))
	defer api.Close()
	server := &Server{options: Options{APIURL: api.URL, Client: http.DefaultClient, Stderr: io.Discard}}
	session := &smtpd.Session{User: `{"email":"me@d.test","token":"tok"}`}
	session.Envelope.MailFrom = &smtpd.Address{Address: "Me@D.test"}
	for _, rcpt := range []string{"To@D.test", "cc@d.test", "Hidden@D.test"} {
		session.Envelope.RcptTo = append(session.Envelope.RcptTo, smtpd.Address{Address: rcpt})
	}
	raw := []byte("From: me@d.test\r\nTo: To@D.test\r\nCc: cc@d.test\r\nSubject: hi\r\n\r\nbody\r\n")
	if err := server.OnData(context.Background(), raw, false, session); err != nil {
		t.Fatal(err)
	}
	want := `{"to":["to@d.test"],"cc":["cc@d.test"],"bcc":["hidden@d.test"],"subject":"hi","text":"body\n"}`
	if gotBody != want || gotAuth != "Bearer tok" {
		t.Fatalf("body = %s auth = %q", gotBody, gotAuth)
	}
	for code, want := range map[int]string{429: "452 Sending rate limit exceeded, try again later", 422: "550 One or more recipients are not allowed", 500: "554 Message rejected by server"} {
		status = code
		err := server.OnData(context.Background(), raw, false, session)
		reply, ok := err.(*smtpd.Reply)
		if !ok || strings.Join([]string{itoa(reply.Code), reply.Message}, " ") != want {
			t.Fatalf("status %d: %v", code, err)
		}
	}
	session.Envelope.MailFrom = &smtpd.Address{Address: "other@d.test"}
	if reply, _ := server.OnData(context.Background(), raw, false, session).(*smtpd.Reply); reply == nil || reply.Code != 550 || reply.Message != "Sender address does not match the authenticated user" {
		t.Fatalf("reply = %+v", reply)
	}
	if reply, _ := server.OnData(context.Background(), raw, true, session).(*smtpd.Reply); reply == nil || reply.Code != 552 {
		t.Fatalf("reply = %+v", reply)
	}
}

func itoa(value int) string { return strconv.Itoa(value) }
