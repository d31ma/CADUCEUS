package smtpclient

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"mesail.dev/packages/smtpclient/sink"
)

type fakeResolver struct {
	records map[string][]*net.MX
	err     error
}

func (resolver fakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return resolver.records[name], resolver.err
}

type routingDialer struct {
	routes map[string]string
	mutex  sync.Mutex
	dialed []string
}

func (dialer *routingDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	dialer.mutex.Lock()
	dialer.dialed = append(dialer.dialed, address)
	target, found := dialer.routes[address]
	dialer.mutex.Unlock()
	if !found {
		return nil, errors.New("no route to " + address)
	}
	return DirectDialer{}.DialContext(ctx, network, target)
}

func startSink(t *testing.T, options sink.Options) *sink.Sink {
	t.Helper()
	server, err := sink.Start(options)
	if err != nil {
		t.Fatalf("start sink: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	return server
}

func selfSigned(t *testing.T) sink.SelfSigned {
	t.Helper()
	certificates, err := sink.NewSelfSigned()
	if err != nil {
		t.Fatalf("certificates: %v", err)
	}
	return certificates
}

func asDeliveryError(t *testing.T, err error) DeliveryError {
	t.Helper()
	var failure DeliveryError
	if !errors.As(err, &failure) {
		t.Fatalf("want DeliveryError, got %T %v", err, err)
	}
	return failure
}

func rejectWith(code int, text string) func(string) (int, string) {
	return func(string) (int, string) { return code, text }
}

type connectProxy struct {
	listener net.Listener
	status   int
	mutex    sync.Mutex
	targets  []string
	headers  []string
}

func startProxy(t *testing.T, status int) *connectProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("proxy listen: %v", err)
	}
	proxy := &connectProxy{listener: listener, status: status}
	t.Cleanup(func() { listener.Close() })
	go proxy.serve()
	return proxy
}

func (proxy *connectProxy) url(user *url.Userinfo) *url.URL {
	return &url.URL{Scheme: "http", Host: proxy.listener.Addr().String(), User: user}
}

func (proxy *connectProxy) serve() {
	for {
		client, err := proxy.listener.Accept()
		if err != nil {
			return
		}
		go proxy.handle(client)
	}
}

func (proxy *connectProxy) handle(client net.Conn) {
	defer client.Close()
	reader := bufio.NewReader(client)
	request, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	proxy.mutex.Lock()
	proxy.targets = append(proxy.targets, request.Method+" "+request.Host)
	proxy.headers = append(proxy.headers, request.Header.Get("Proxy-Authorization"))
	proxy.mutex.Unlock()
	if proxy.status != http.StatusOK {
		io.WriteString(client, "HTTP/1.1 403 Forbidden\r\nContent-Length: 6\r\n\r\ndenied")
		return
	}
	upstream, err := net.Dial("tcp", request.Host)
	if err != nil {
		io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	defer upstream.Close()
	io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n")
	go io.Copy(upstream, reader)
	io.Copy(client, upstream)
}

func (proxy *connectProxy) seen() ([]string, []string) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	return append([]string(nil), proxy.targets...), append([]string(nil), proxy.headers...)
}

func silentListener(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	accepted := make(chan net.Conn, 16)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				close(accepted)
				return
			}
			accepted <- connection
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		for connection := range accepted {
			connection.Close()
		}
	})
	return listener.Addr().String()
}

func closedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}
