// Package submission is the authenticated message-submission server on 587
// (STARTTLS) and 465 (implicit TLS). An accepted message is pushed through
// the API's `POST /send` with the submitting user's bearer token.
//
// It ports the former background/SubmissionServer.js. Authentication is refused for
// now, exactly as the JavaScript did: a CADUCEUS session needs multi-factor
// assurance and a mail client offers one secret, so the DATA path is reached
// only once application passwords ship (Phase 6).
package submission

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
)

// RefusalMessage is the AUTH answer while application passwords do not exist.
const RefusalMessage = "SMTP submission requires an application password, which this release does not issue yet. " +
	"Send through the web client until application passwords ship."

// Options configures one submission listener. Port 0 binds an ephemeral
// port; the entry point passes 587 or SMTP_SUBMISSION_TLS_PORT.
type Options struct {
	Port              int
	Secure            bool
	Host              string
	APIURL            string
	Hostname          string
	MaxSize           int64
	TLS               *mailtls.Material
	AllowInsecureAuth bool
	// Authenticate answers the session identity, JSON {"email","token"}. The
	// default refuses every attempt with RefusalMessage.
	Authenticate func(ctx context.Context, auth smtpd.Auth) (string, error)
	Client       *http.Client
	Stdout       io.Writer
	Stderr       io.Writer
	SMTP         func(*smtpd.Options)
}

// FromSettings answers the options SubmissionServer's constructor derived
// from the settings for one port.
func FromSettings(port int, secure bool, apiURL string) Options {
	if apiURL == "" {
		portValue := settings.Value("PORT")
		if portValue == "" {
			portValue = "8080"
		}
		apiURL = "http://127.0.0.1:" + portValue
	}
	maxSize, _ := settings.Number(settings.Or("SMTP_MAX_SIZE", "26214400"))
	return Options{
		Port:     port,
		Secure:   secure,
		Host:     settings.Or("SMTP_SUBMISSION_HOST", "0.0.0.0"),
		APIURL:   strings.TrimSuffix(apiURL, "/"),
		Hostname: settings.Or("MAIL_HOSTNAME", "localhost"),
		MaxSize:  int64(maxSize),
		TLS:      mailtls.Load(),
	}
}

// Server is one started submission listener.
type Server struct {
	options Options
	smtp    *smtpd.Server
	addr    net.Addr
	errs    chan error
}

// Start binds the listener. It refuses to run without TLS unless
// AllowInsecureAuth is set, so credentials never cross the wire in clear.
func Start(options Options) (*Server, error) {
	if options.TLS == nil && !options.AllowInsecureAuth {
		return nil, errors.New("SMTP submission requires TLS (MAIL_TLS_CERT/KEY)")
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
		Name:        options.Hostname,
		Banner:      options.Hostname + " ESMTP CADUCEUS submission",
		Size:        options.MaxSize,
		Secure:      options.Secure,
		AuthMethods: []string{"PLAIN", "LOGIN"},
		// Require STARTTLS before AUTH on 587 (465 is already TLS).
		AllowInsecureAuth: options.AllowInsecureAuth,
		TLSConfig:         tlsConfig,
		OnAuth: func(ctx context.Context, auth smtpd.Auth, _ *smtpd.Session) (string, error) {
			if options.Authenticate != nil {
				return options.Authenticate(ctx, auth)
			}
			return "", &smtpd.Reply{Message: RefusalMessage}
		},
		OnData: server.OnData,
		OnError: func(err error) {
			fmt.Fprintf(options.Stderr, "[smtp-sub] server error: %v\n", err)
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
			fmt.Fprintf(options.Stderr, "[smtp-sub] server error: %v\n", err)
			server.errs <- err
		}
		close(server.errs)
	}()
	mode := "STARTTLS"
	if options.Secure {
		mode = "implicit TLS"
	}
	fmt.Fprintf(options.Stdout, "[smtp-sub] submission server listening on %s:%d (%s)\n", options.Host, options.Port, mode)
	return server, nil
}

// Addr answers the bound address.
func (server *Server) Addr() net.Addr { return server.addr }

// Errors delivers a listener failure after startup, then closes.
func (server *Server) Errors() <-chan error { return server.errs }

// Close stops the listener and waits for open sessions.
func (server *Server) Close(ctx context.Context) error { return server.smtp.Close(ctx) }

type identity struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

type sendPayload struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
}

func lower(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value = strings.ToLower(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// OnData parses an authenticated submission and pushes it through
// `POST /send`, refusing an envelope sender other than the authenticated one.
func (server *Server) OnData(ctx context.Context, raw []byte, sizeExceeded bool, session *smtpd.Session) error {
	if sizeExceeded {
		return &smtpd.Reply{Code: 552, Message: "Message exceeds maximum permitted size"}
	}
	err := server.submit(ctx, raw, session)
	if err == nil {
		return nil
	}
	var reply *smtpd.Reply
	if errors.As(err, &reply) {
		return reply
	}
	fmt.Fprintf(server.options.Stderr, "[smtp-sub] submission failed: %v\n", err)
	return &smtpd.Reply{Code: 451, Message: "Temporary failure, try again later"}
}

func (server *Server) submit(ctx context.Context, raw []byte, session *smtpd.Session) error {
	user := session.User
	if user == "" {
		user = "{}"
	}
	var who identity
	if err := json.Unmarshal([]byte(user), &who); err != nil {
		return err
	}
	// An authenticated client must not spoof another address.
	envelopeFrom := ""
	if session.Envelope.MailFrom != nil {
		envelopeFrom = strings.ToLower(session.Envelope.MailFrom.Address)
	}
	if envelopeFrom != "" && envelopeFrom != who.Email {
		return &smtpd.Reply{Code: 550, Message: "Sender address does not match the authenticated user"}
	}
	parsed, err := mimeparse.Parse(raw)
	if err != nil {
		return err
	}
	to, cc := lower(parsed.To), lower(parsed.Cc)
	// BCC recipients appear in the SMTP envelope but not the headers.
	headerRecipients := map[string]bool{}
	for _, address := range append(append([]string{}, to...), cc...) {
		headerRecipients[address] = true
	}
	var bcc []string
	for _, recipient := range session.Envelope.RcptTo {
		if address := strings.ToLower(recipient.Address); !headerRecipients[address] {
			bcc = append(bcc, address)
		}
	}
	subject := parsed.Subject
	if subject == "" {
		subject = "(no subject)"
	}
	body, err := json.Marshal(sendPayload{To: to, Cc: cc, Bcc: bcc, Subject: subject, Text: parsed.Text, HTML: parsed.HTML})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.options.APIURL+"/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+who.Token)
	response, err := server.options.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	switch {
	case response.StatusCode >= 200 && response.StatusCode <= 299:
		return nil
	case response.StatusCode == http.StatusTooManyRequests:
		return &smtpd.Reply{Code: 452, Message: "Sending rate limit exceeded, try again later"}
	case response.StatusCode == http.StatusUnprocessableEntity:
		return &smtpd.Reply{Code: 550, Message: "One or more recipients are not allowed"}
	default:
		return &smtpd.Reply{Code: 554, Message: "Message rejected by server"}
	}
}
