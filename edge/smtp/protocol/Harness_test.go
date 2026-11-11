package protocol

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"mesail.dev/edge/smtp/reception"
)

type fakeDirectory struct {
	hostedDomains map[string]bool
	knownPrefixes []string
}

func (directory fakeDirectory) Lookup(_ context.Context, address string) (reception.Address, error) {
	local, domain, _ := strings.Cut(address, "@")
	if domain == "failing.test" {
		return reception.Address{}, errors.New("directory answered HTTP 503")
	}
	if !directory.hostedDomains[domain] {
		return reception.Address{}, reception.DomainNotHosted{}
	}
	for _, prefix := range directory.knownPrefixes {
		if strings.HasPrefix(local, prefix) {
			return reception.Address{Address: address, DomainId: domain, Kind: "member", TargetId: "mem_" + local}, nil
		}
	}
	return reception.Address{}, reception.UnknownAddress{}
}

type fakeDelivery struct {
	mutex        sync.Mutex
	transactions []reception.Transaction
	err          error
}

func (delivery *fakeDelivery) Deliver(_ context.Context, transaction reception.Transaction) (reception.Receipt, error) {
	delivery.mutex.Lock()
	defer delivery.mutex.Unlock()
	delivery.transactions = append(delivery.transactions, transaction)
	if delivery.err != nil {
		return reception.Receipt{}, delivery.err
	}
	return reception.Receipt{InboundIds: []string{"in_1"}}, nil
}

func (delivery *fakeDelivery) delivered() []reception.Transaction {
	delivery.mutex.Lock()
	defer delivery.mutex.Unlock()
	return append([]reception.Transaction{}, delivery.transactions...)
}

type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (locked *lockedBuffer) Write(data []byte) (int, error) {
	locked.mutex.Lock()
	defer locked.mutex.Unlock()
	return locked.buffer.Write(data)
}

func (locked *lockedBuffer) String() string {
	locked.mutex.Lock()
	defer locked.mutex.Unlock()
	return locked.buffer.String()
}

type harness struct {
	address  string
	delivery *fakeDelivery
	logs     *lockedBuffer
	server   *Server
}

func newHarness(t *testing.T, change func(*Settings)) *harness {
	t.Helper()
	delivery := &fakeDelivery{}
	logs := &lockedBuffer{}
	settings := Settings{
		Hostname:       "mx.mesail.test",
		MaxSize:        1024,
		MaxConnections: 10,
		Directory:      fakeDirectory{hostedDomains: map[string]bool{"example.com": true, "example.org": true, "failing.test": true}, knownPrefixes: []string{"ana", "bob", "cy", "any"}},
		Delivery:       delivery,
		Log:            log.New(logs, "", 0),
	}
	change(&settings)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(settings)
	go server.Serve(listener)
	t.Cleanup(func() { server.Shutdown(context.Background()) })
	return &harness{address: listener.Addr().String(), delivery: delivery, logs: logs, server: server}
}

func noChange(_ *Settings) {}

type rawClient struct {
	t        *testing.T
	text     *textproto.Conn
	greeting string
}

func dialRaw(t *testing.T, address string) *rawClient {
	t.Helper()
	text, err := textproto.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { text.Close() })
	client := &rawClient{t: t, text: text}
	_, client.greeting = client.response()
	return client
}

func (client *rawClient) command(line string) (int, string) {
	client.t.Helper()
	if err := client.text.PrintfLine("%s", line); err != nil {
		client.t.Fatal(err)
	}
	return client.response()
}

func (client *rawClient) response() (int, string) {
	client.t.Helper()
	code, message, err := client.text.ReadResponse(0)
	var protocolError *textproto.Error
	if err != nil && !errors.As(err, &protocolError) {
		client.t.Fatal(err)
	}
	return code, message
}

func (client *rawClient) expect(line string, wantCode int, wantText string) string {
	client.t.Helper()
	code, message := client.command(line)
	if code != wantCode || !strings.Contains(message, wantText) {
		client.t.Fatalf("%s: want %d %q, got %d %q", line, wantCode, wantText, code, message)
	}
	return message
}

func (client *rawClient) sendData(content string) (int, string) {
	client.t.Helper()
	writer := client.text.DotWriter()
	if _, err := writer.Write([]byte(content)); err != nil {
		client.t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		client.t.Fatal(err)
	}
	return client.response()
}

func (client *rawClient) pipelineRecipients(count int) {
	client.t.Helper()
	lines := strings.Builder{}
	for number := range count {
		lines.WriteString("RCPT TO:<any" + strings.Repeat("x", number) + "@example.com>\r\n")
	}
	client.text.W.WriteString(lines.String())
	client.text.W.Flush()
	for range count {
		client.response()
	}
}

func (client *rawClient) repeat(line string, count int) {
	client.t.Helper()
	for range count {
		client.command(line)
	}
}
