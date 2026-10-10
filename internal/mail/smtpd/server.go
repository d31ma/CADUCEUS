// Package smtpd is a small SMTP server on the standard library.
//
// It reproduces the parts of the Node `smtp-server` package (v3) that the
// CADUCEUS inbound MX and submission servers used: the greeting and command
// replies, EHLO features, STARTTLS and implicit TLS, AUTH PLAIN and LOGIN,
// size limits, MAIL/RCPT address checks, DATA dot-unstuffing, and the
// connection guards (early talkers, HTTP requests, unrecognized and
// unauthenticated command limits, idle timeout). smtp-server's defaults are
// kept: enhanced status codes, DSN and REQUIRETLS are hidden.
package smtpd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Reply is an SMTP failure with an explicit response code, the counterpart
// of a JavaScript error with `responseCode`.
type Reply struct {
	Code    int
	Message string
}

func (reply *Reply) Error() string { return reply.Message }

// Auth is one AUTH attempt.
type Auth struct {
	Method   string
	Username string
	Password string
	Authcid  string
	Authzid  string
}

// Session is the state of one SMTP connection that handlers see.
type Session struct {
	ID                string
	RemoteAddress     string
	RemotePort        int
	LocalAddress      string
	LocalPort         int
	ClientHostname    string
	HostNameAppearsAs string
	OpeningCommand    string
	Secure            bool
	ServerName        string
	// User is the identity OnAuth answered; empty when not authenticated.
	User     string
	Envelope Envelope
}

// Envelope is the current mail transaction.
type Envelope struct {
	// MailFrom is nil until MAIL FROM is accepted.
	MailFrom *Address
	RcptTo   []Address
	BodyType string
	SMTPUTF8 bool
}

// Options configures a Server.
type Options struct {
	// Name is the server hostname used in replies (default os.Hostname()).
	Name string
	// Banner is appended to the greeting after "<name> ESMTP".
	Banner string
	// Size is the maximum message size in bytes; 0 means no limit.
	Size int64
	// Secure makes every connection implicit TLS (port 465).
	Secure bool
	// TLSConfig enables STARTTLS (or implicit TLS). Without it STARTTLS is
	// not offered.
	TLSConfig    *tls.Config
	HideSTARTTLS bool
	// DisabledCommands lists commands treated as unrecognized, such as AUTH.
	DisabledCommands []string
	// AuthMethods defaults to LOGIN and PLAIN, as smtp-server's did.
	AuthMethods       []string
	AuthOptional      bool
	AllowInsecureAuth bool
	// MaxUnauthenticatedCommands defaults to 10.
	MaxUnauthenticatedCommands int

	// OnAuth answers the authenticated user, or a *Reply (default 535).
	OnAuth func(ctx context.Context, auth Auth, session *Session) (string, error)
	// OnMailFrom and OnRcptTo may refuse a path with a *Reply (default 550).
	OnMailFrom func(ctx context.Context, address Address, session *Session) error
	OnRcptTo   func(ctx context.Context, address Address, session *Session) error
	// OnData receives the dot-unstuffed message (CRLF kept), truncated at Size
	// when sizeExceeded is true. A *Reply error sets the response (default
	// 450); a nil error answers "250 OK: message queued".
	OnData func(ctx context.Context, raw []byte, sizeExceeded bool, session *Session) error
	// OnError receives connection-level failures (TLS handshakes, sockets).
	OnError func(error)

	SocketTimeout time.Duration
	CloseTimeout  time.Duration
	// ReadyDelay is how long a client must stay quiet before the greeting
	// (default 100 ms, the early-talker guard).
	ReadyDelay           time.Duration
	DisableReverseLookup bool
	LookupAddr           func(ctx context.Context, address string) ([]string, error)
}

// Server accepts SMTP connections.
type Server struct {
	options  Options
	mu       sync.Mutex
	listener net.Listener
	conns    map[*connection]struct{}
	closing  bool
	wg       sync.WaitGroup
	nextID   uint64
	done     chan struct{}
	closeErr error
}

// New returns a server with smtp-server's defaults applied.
func New(options Options) *Server {
	if options.Name == "" {
		options.Name, _ = os.Hostname()
	}
	if len(options.AuthMethods) == 0 {
		options.AuthMethods = []string{"LOGIN", "PLAIN"}
	}
	for index, method := range options.AuthMethods {
		options.AuthMethods[index] = strings.ToUpper(strings.TrimSpace(method))
	}
	for index, command := range options.DisabledCommands {
		options.DisabledCommands[index] = strings.ToUpper(strings.TrimSpace(command))
	}
	if options.MaxUnauthenticatedCommands == 0 {
		options.MaxUnauthenticatedCommands = 10
	}
	if options.SocketTimeout == 0 {
		options.SocketTimeout = 60 * time.Second
	}
	if options.CloseTimeout == 0 {
		options.CloseTimeout = 30 * time.Second
	}
	if options.ReadyDelay == 0 {
		options.ReadyDelay = 100 * time.Millisecond
	}
	if options.LookupAddr == nil {
		options.LookupAddr = net.DefaultResolver.LookupAddr
	}
	return &Server{options: options, conns: map[*connection]struct{}{}, done: make(chan struct{})}
}

// Listen binds address (host:port) and answers the listener; bind failures
// such as EADDRINUSE are returned instead of hanging startup.
func (server *Server) Listen(address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	server.mu.Lock()
	server.listener = listener
	server.mu.Unlock()
	return listener, nil
}

// Serve accepts connections until Close. It answers nil after Close and the
// accept error otherwise.
func (server *Server) Serve(listener net.Listener) error {
	server.mu.Lock()
	server.listener = listener
	server.mu.Unlock()
	for {
		raw, err := listener.Accept()
		if err != nil {
			server.mu.Lock()
			closing := server.closing
			server.mu.Unlock()
			if closing || errors.Is(err, net.ErrClosed) {
				return nil
			}
			var temporary interface{ Temporary() bool }
			if errors.As(err, &temporary) && temporary.Temporary() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		server.mu.Lock()
		if server.closing {
			server.mu.Unlock()
			_ = raw.Close()
			continue
		}
		server.nextID++
		conn := newConnection(server, raw, server.nextID)
		server.conns[conn] = struct{}{}
		server.wg.Add(1)
		server.mu.Unlock()
		go func() {
			defer server.wg.Done()
			conn.serve()
			server.mu.Lock()
			delete(server.conns, conn)
			server.mu.Unlock()
		}()
	}
}

// Close stops accepting connections. Open connections get "421 Server
// shutting down" at their next command; any still open after CloseTimeout
// (or when ctx ends) are sent that reply and closed.
func (server *Server) Close(ctx context.Context) error {
	server.mu.Lock()
	if server.closing {
		server.mu.Unlock()
		<-server.done
		return server.closeErr
	}
	server.closing = true
	listener := server.listener
	server.mu.Unlock()
	if listener != nil {
		server.closeErr = listener.Close()
		if errors.Is(server.closeErr, net.ErrClosed) {
			server.closeErr = nil
		}
	}
	finished := make(chan struct{})
	go func() {
		server.wg.Wait()
		close(finished)
	}()
	timer := time.NewTimer(server.options.CloseTimeout)
	defer timer.Stop()
	select {
	case <-finished:
	case <-timer.C:
		server.terminateAll()
		<-finished
	case <-ctx.Done():
		server.terminateAll()
		<-finished
	}
	close(server.done)
	return server.closeErr
}

func (server *Server) terminateAll() {
	server.mu.Lock()
	conns := make([]*connection, 0, len(server.conns))
	for conn := range server.conns {
		conns = append(conns, conn)
	}
	server.mu.Unlock()
	for _, conn := range conns {
		conn.terminate()
	}
}

func (server *Server) isClosing() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.closing
}

func (server *Server) reportError(err error) {
	if server.options.OnError != nil && err != nil {
		server.options.OnError(err)
	}
}

func (server *Server) supported(command string) bool {
	for _, disabled := range server.options.DisabledCommands {
		if disabled == command {
			return false
		}
	}
	if command == "STARTTLS" && server.options.TLSConfig == nil {
		return false
	}
	_, ok := handlers[command]
	return ok
}

func (server *Server) authMethod(method string) bool {
	for _, candidate := range server.options.AuthMethods {
		if candidate == method {
			return true
		}
	}
	return false
}

func connectionID(sequence uint64) string {
	return fmt.Sprintf("%08x%04x", time.Now().UnixNano()&0xffffffff, sequence&0xffff)
}
