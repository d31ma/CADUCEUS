// Package inbound is the standalone MX: it accepts mail on port 25, verifies
// SPF/DKIM/DMARC, and hands each hosted recipient's copy to the API's signed
// `POST /inbound/webhook`, which owns delivery.
//
// It ports the former background/InboundSmtpServer.js.
package inbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/d31ma/caduceus/internal/mail/mailtls"
	"github.com/d31ma/caduceus/internal/mail/mimeparse"
	"github.com/d31ma/caduceus/internal/mail/smtpd"
	"github.com/d31ma/caduceus/internal/runtime/settings"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

// AuthInput is what the verifier learns about the sending connection.
type AuthInput struct {
	IP     string
	Helo   string
	Sender string
	MTA    string
}

// AuthOutcome is the part of a mailauth result the MX reads.
type AuthOutcome struct {
	// Headers is the Received-SPF and Authentication-Results block,
	// CRLF-terminated, to prepend to the message.
	Headers  string
	SPF      string
	DMARC    string
	DKIMPass bool
}

// Authenticator verifies SPF, DKIM and DMARC for one message.
type Authenticator func(ctx context.Context, raw []byte, input AuthInput) (*AuthOutcome, error)

// Options configures the MX. Zero values take the settings defaults through
// FromSettings.
type Options struct {
	Port         int
	Host         string
	APIURL       string
	Secret       string
	Hostname     string
	MaxSize      int64
	TLS          *mailtls.Material
	VerifyAuth   bool
	Authenticate Authenticator
	Client       *http.Client
	Stdout       io.Writer
	Stderr       io.Writer
	// SMTP overrides smtpd options in tests (timeouts, reverse lookup).
	SMTP func(*smtpd.Options)
}

// FromSettings answers the options InboundSmtpServer's constructor derived
// from the deployment settings. apiURL, when not empty, wins over
// CADUCEUS_API_URL.
func FromSettings(apiURL string) (Options, error) {
	port, _ := settings.Number(settings.Or("SMTP_INBOUND_PORT", "25"))
	if apiURL == "" {
		apiURL = settings.Or("CADUCEUS_API_URL", "http://127.0.0.1:"+portSetting())
	}
	maxSize, _ := settings.Number(settings.Or("SMTP_MAX_SIZE", "26214400"))
	options := Options{
		Port:       int(port),
		Host:       settings.Or("SMTP_INBOUND_HOST", "0.0.0.0"),
		APIURL:     strings.TrimSuffix(apiURL, "/"),
		Secret:     settings.Value("INBOUND_WEBHOOK_SECRET"),
		Hostname:   settings.Or("MAIL_HOSTNAME", "caduceus"),
		MaxSize:    int64(maxSize),
		TLS:        mailtls.Load(),
		VerifyAuth: settings.Value("MAIL_AUTH_VERIFY") != "off",
	}
	if options.Secret == "" {
		return Options{}, errors.New("INBOUND_WEBHOOK_SECRET is required to run the inbound SMTP server")
	}
	return options, nil
}

func portSetting() string {
	if port := settings.Value("PORT"); port != "" {
		return port
	}
	return "8080"
}

// Server is a started MX.
type Server struct {
	options Options
	smtp    *smtpd.Server
	addr    net.Addr
	errs    chan error
}

// Start binds the listener and answers once it accepts connections. Bind
// failures such as EADDRINUSE are returned.
func Start(options Options) (*Server, error) {
	if options.Secret == "" {
		return nil, errors.New("INBOUND_WEBHOOK_SECRET is required to run the inbound SMTP server")
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	if options.Client == nil {
		options.Client = http.DefaultClient
	}
	tlsConfig, err := mailtls.Config(options.TLS)
	if err != nil {
		return nil, err
	}
	server := &Server{options: options, errs: make(chan error, 1)}
	smtpOptions := smtpd.Options{
		Name:   options.Hostname,
		Banner: options.Hostname + " ESMTP CADUCEUS",
		Size:   options.MaxSize,
		// A public MX accepts mail without authenticating the sending server;
		// authenticated submission is a separate concern on 587/465.
		AuthOptional:     true,
		DisabledCommands: []string{"AUTH"},
		TLSConfig:        tlsConfig,
		HideSTARTTLS:     tlsConfig == nil,
		OnData:           server.onData,
		OnError: func(err error) {
			fmt.Fprintf(options.Stderr, "[smtp-in] server error: %v\n", err)
		},
	}
	if options.SMTP != nil {
		options.SMTP(&smtpOptions)
	}
	server.smtp = smtpd.New(smtpOptions)
	listener, err := server.smtp.Listen(net.JoinHostPort(options.Host, strconv.Itoa(options.Port)))
	if err != nil {
		return nil, err
	}
	server.addr = listener.Addr()
	go func() {
		if err := server.smtp.Serve(listener); err != nil {
			fmt.Fprintf(options.Stderr, "[smtp-in] server error: %v\n", err)
			server.errs <- err
		}
		close(server.errs)
	}()
	starttls := "off"
	if tlsConfig != nil {
		starttls = "on"
	}
	fmt.Fprintf(options.Stdout, "[smtp-in] inbound SMTP server listening on %s:%d → %s/inbound/webhook (STARTTLS %s)\n",
		options.Host, options.Port, options.APIURL, starttls)
	return server, nil
}

// Addr answers the bound address.
func (server *Server) Addr() net.Addr { return server.addr }

// Errors delivers a listener failure after startup, then closes.
func (server *Server) Errors() <-chan error { return server.errs }

// Close stops accepting mail and waits for open sessions.
func (server *Server) Close(ctx context.Context) error { return server.smtp.Close(ctx) }

func (server *Server) onData(ctx context.Context, raw []byte, sizeExceeded bool, session *smtpd.Session) error {
	failure, err := server.ProcessMessage(ctx, raw, session, sizeExceeded)
	if err != nil {
		fmt.Fprintf(server.options.Stderr, "[smtp-in] delivery error: %v\n", err)
		return &smtpd.Reply{Code: 451, Message: "Temporary local delivery failure, try again later"}
	}
	if failure != nil {
		return failure
	}
	return nil
}

// ProcessMessage authenticates one received message and fans it out to every
// hosted recipient. It answers the SMTP failure, or nil on success; err is an
// unexpected delivery error (answered 451 by the caller).
func (server *Server) ProcessMessage(ctx context.Context, raw []byte, session *smtpd.Session, sizeExceeded bool) (*smtpd.Reply, error) {
	if sizeExceeded {
		return &smtpd.Reply{Code: 552, Message: "Message exceeds maximum permitted size"}, nil
	}
	parsed, _ := mimeparse.Parse(raw)
	subject := ""
	if parsed != nil {
		subject = parsed.Subject
	}
	if subject == "" {
		subject = "(no subject)"
	}
	envelopeFrom := ""
	if session.Envelope.MailFrom != nil {
		envelopeFrom = session.Envelope.MailFrom.Address
	}
	sender := envelopeFrom
	if sender == "" && parsed != nil && len(parsed.From) > 0 {
		sender = parsed.From[0]
	}

	// Verify SPF/DKIM/DMARC and prepend the Authentication-Results header so
	// downstream tooling (and the stored message) can see the result.
	auth := authentication{summary: "auth=off"}
	if server.options.VerifyAuth {
		auth = server.authenticateMessage(ctx, raw, session)
	}
	body := auth.headerText + mimeparse.NodeUTF8(raw)

	// Deliver a copy to each hosted recipient. The pipeline generates a
	// distinct stored id per recipient, so shared Message-IDs never clash.
	anyHosted := false
	for _, recipient := range session.Envelope.RcptTo {
		result, err := server.deliverViaWebhook(ctx, webhookPayload{
			Recipient: recipient.Address, Sender: sender, Subject: subject, Body: body,
			SpamVerdict: auth.spamVerdict, SpamScore: auth.spamScore,
		})
		if err != nil {
			return nil, err
		}
		if result.tooLarge {
			// The server refuses request bodies over its limit (Tachyon's
			// is 1 MiB). Retrying cannot succeed, so refuse permanently.
			return &smtpd.Reply{Code: 552, Message: "Message exceeds maximum permitted size"}, nil
		}
		if !result.ok {
			return &smtpd.Reply{Code: 451, Message: "Temporary local delivery failure, try again later"}, nil
		}
		// "domain not configured" means we do not host this recipient; a real
		// MX refuses rather than silently blackholing it.
		if result.accepted || result.reason != "domain not configured" {
			anyHosted = true
		}
	}
	if !anyHosted {
		return &smtpd.Reply{Code: 550, Message: "No such user here"}, nil
	}
	return nil, nil
}

type webhookPayload struct {
	Recipient   string `json:"recipient"`
	Sender      string `json:"sender"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	SpamVerdict string `json:"spamVerdict,omitempty"`
	SpamScore   int    `json:"spamScore,omitempty"`
}

type webhookResult struct {
	ok       bool
	accepted bool
	tooLarge bool
	reason   string
}

func marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

// deliverViaWebhook POSTs one recipient's copy to the local webhook, signed
// with the shared HMAC secret, and answers the pipeline's decision.
func (server *Server) deliverViaWebhook(ctx context.Context, payload webhookPayload) (webhookResult, error) {
	body, err := marshal(payload)
	if err != nil {
		return webhookResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.options.APIURL+"/inbound/webhook", bytes.NewReader(body))
	if err != nil {
		return webhookResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Caduceus-Signature", tickworker.Sign(server.options.Secret, body))
	response, err := server.options.Client.Do(request)
	if err != nil {
		return webhookResult{}, err
	}
	defer response.Body.Close()
	answer, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return webhookResult{ok: false, tooLarge: response.StatusCode == http.StatusRequestEntityTooLarge, reason: fmt.Sprintf("webhook %d", response.StatusCode)}, nil
	}
	var data map[string]any
	_ = json.Unmarshal(answer, &data)
	accepted := true
	if value, ok := data["accepted"].(bool); ok && !value {
		accepted = false
	}
	reason, _ := data["reason"].(string)
	return webhookResult{ok: true, accepted: accepted, reason: reason}, nil
}

type authentication struct {
	headerText  string
	spamVerdict string
	spamScore   int
	summary     string
}

// authenticateMessage derives the spam signal the delivery pipeline
// understands: a DMARC failure is an authenticated spoof (verdict fail, score
// 6) and a hard SPF failure on an unsigned message adds suspicion (score 3).
// A verification error never fails delivery.
func (server *Server) authenticateMessage(ctx context.Context, raw []byte, session *smtpd.Session) authentication {
	if server.options.Authenticate == nil {
		return authentication{summary: "auth=off"}
	}
	helo := session.HostNameAppearsAs
	if helo == "" {
		helo = session.ClientHostname
	}
	sender := ""
	if session.Envelope.MailFrom != nil {
		sender = session.Envelope.MailFrom.Address
	}
	result, err := server.options.Authenticate(ctx, raw, AuthInput{
		IP: session.RemoteAddress, Helo: helo, Sender: sender, MTA: server.options.Hostname,
	})
	if err != nil || result == nil {
		message := "no result"
		if err != nil {
			message = err.Error()
		}
		fmt.Fprintf(server.options.Stderr, "[smtp-in] mailauth verification failed (delivering unverified): %s\n", message)
		return authentication{summary: "auth=error"}
	}
	outcome := authentication{headerText: result.Headers}
	switch {
	case result.DMARC == "fail":
		outcome.spamVerdict, outcome.spamScore = "fail", 6
	case result.SPF == "fail" && !result.DKIMPass:
		outcome.spamScore = 3
	}
	spf, dmarc, dkim := result.SPF, result.DMARC, "fail/none"
	if spf == "" {
		spf = "n/a"
	}
	if dmarc == "" {
		dmarc = "n/a"
	}
	if result.DKIMPass {
		dkim = "pass"
	}
	outcome.summary = fmt.Sprintf("spf=%s dkim=%s dmarc=%s", spf, dkim, dmarc)
	return outcome
}
