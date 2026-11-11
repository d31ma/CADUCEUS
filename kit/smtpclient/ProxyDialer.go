package smtpclient

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

type ProxyDialer struct {
	ProxyUrl *url.URL
	Timeout  time.Duration
}

func (proxy ProxyDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	connection, err := DirectDialer{Timeout: proxy.Timeout}.DialContext(ctx, network, proxy.proxyAddress())
	if err != nil {
		return nil, err
	}
	return proxy.connect(ctx, connection, address)
}

func (ProxyDialer) defaultPort() string { return "80" }

func (proxy ProxyDialer) proxyAddress() string {
	if proxy.ProxyUrl.Port() == "" {
		return net.JoinHostPort(proxy.ProxyUrl.Hostname(), proxy.defaultPort())
	}
	return proxy.ProxyUrl.Host
}

func (proxy ProxyDialer) connect(ctx context.Context, connection net.Conn, address string) (net.Conn, error) {
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	defer stop()
	reader := bufio.NewReader(connection)
	response, err := proxy.handshake(connection, reader, address)
	if err != nil {
		connection.Close()
		return nil, err
	}
	return proxy.established(tunnel{Conn: connection, reader: reader}, response, address)
}

func (proxy ProxyDialer) handshake(connection net.Conn, reader *bufio.Reader, address string) (*http.Response, error) {
	request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address, Header: proxy.headers()}
	if err := request.Write(connection); err != nil {
		return nil, err
	}
	return http.ReadResponse(reader, request)
}

func (proxy ProxyDialer) headers() http.Header {
	headers := http.Header{}
	if proxy.ProxyUrl.User == nil {
		return headers
	}
	password, _ := proxy.ProxyUrl.User.Password()
	credentials := proxy.ProxyUrl.User.Username() + ":" + password
	headers.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	return headers
}

func (proxy ProxyDialer) established(connection tunnel, response *http.Response, address string) (net.Conn, error) {
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		connection.Close()
		return nil, fmt.Errorf("proxy refused CONNECT to %s: %s", address, response.Status)
	}
	return connection, nil
}
