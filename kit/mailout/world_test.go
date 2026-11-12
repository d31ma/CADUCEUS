package mailout

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/smtpclient/sink"
)

type world struct {
	t           *testing.T
	documents   *memoryDocuments
	server      *httptest.Server
	relay       *sink.Sink
	folder      string
	lock        sync.Mutex
	handOffs    []InboundEnvelope
	answers     []int
	eventAnswer int
	pushed      []events.Envelope
	calls       []string
	domains     map[string]Domain
	sources     map[string]MessageSource
	rejected    string
	rejectCode  int
	rejectText  string
	rejecting   bool
	settings    Settings
	desk        Desk
}

func newWorld(t *testing.T, adjust func(*world)) *world {
	t.Setenv("TMPDIR", t.TempDir())
	w := &world{t: t, documents: newMemoryDocuments(), folder: t.TempDir(), eventAnswer: http.StatusOK, rejecting: true}
	w.domains = map[string]Domain{
		"example.com": {DomainId: "example.com", Name: "example.com", State: "active", DkimSelector: "mesail"},
		"example.org": {DomainId: "example.org", Name: "example.org", State: "active", DkimSelector: "mesail"},
	}
	w.sources = map[string]MessageSource{}
	for _, domainId := range []string{"example.com", "example.org"} {
		w.writeKey(domainId)
	}
	w.server = w.services()
	t.Cleanup(w.server.Close)
	relay, err := sink.Start(sink.Options{RejectRecipient: w.reject})
	if err != nil {
		t.Fatal(err)
	}
	w.relay = relay
	t.Cleanup(func() { relay.Close() })
	w.settings = Settings{
		MailInUrl: w.server.URL, MessagingUrl: w.server.URL, NotifyUrl: w.server.URL, DirectoryUrl: w.server.URL,
		RelayAddress: relay.Address(), SecretFolder: filepath.Join(w.folder, "secrets"), BlobFolder: filepath.Join(w.folder, "blobs"),
	}
	if adjust != nil {
		adjust(w)
	}
	w.desk = DeskFactory{Environment: w.environment}.Desk(w.documents)
	w.desk.Gate = tokenGate{}
	return w
}

func (w *world) environment(name string) string {
	if name != "YON_APPLICATION_JSON" {
		return ""
	}
	snapshot := map[string]any{"yon": map[string]any{
		"mailout": w.settings,
		"outbox": map[string]any{
			"producer":    "mail-out",
			"subscribers": []map[string]string{{"name": "messaging", "url": w.server.URL + "/api/v1/messaging/events"}},
			"retry":       map[string]int{"baseDelaySeconds": 1, "maxDelaySeconds": 1, "maxAttempts": 2},
		},
		"oidc":    map[string]string{"issuer": "https://mail.mesail.test/api/v1/identity", "jwks": "https://mail.mesail.test/api/v1/identity/jwks", "audience": "mesail-api", "serviceTokens": w.server.URL + "/api/v1/identity/service-token"},
		"service": map[string]any{"client": map[string]string{"secret": "test-value"}},
	}}
	raw, _ := json.Marshal(snapshot)
	return string(raw)
}

func (w *world) writeKey(domainId string) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	encoded, _ := x509.MarshalPKCS8PrivateKey(private)
	path := filepath.Join(w.folder, "secrets", "dkim", domainId, "mesail.pem")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600)
}

func (w *world) services() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mail-in/messages", w.mailIn)
	mux.HandleFunc("GET /api/v1/directory/domains/{domainId}", w.directory)
	mux.HandleFunc("GET /api/v1/{producer}/messages/{messageId}/source", w.source)
	mux.HandleFunc("POST /api/v1/messaging/events", w.messagingEvents)
	mux.HandleFunc("POST /api/v1/identity/service-token", w.serviceToken)
	return httptest.NewServer(mux)
}

func (w *world) record(name string, request *http.Request) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.calls = append(w.calls, name+" "+tokenRealm(request.Header.Get("Authorization")))
}

func (w *world) serviceToken(writer http.ResponseWriter, request *http.Request) {
	asked := map[string]string{}
	_ = json.NewDecoder(request.Body).Decode(&asked)
	_ = json.NewEncoder(writer).Encode(map[string]any{"accessToken": "token-" + asked["domainId"], "expiresIn": 60})
}

func (w *world) mailIn(writer http.ResponseWriter, request *http.Request) {
	w.record("mail-in", request)
	envelope := InboundEnvelope{}
	_ = json.NewDecoder(request.Body).Decode(&envelope)
	w.lock.Lock()
	defer w.lock.Unlock()
	w.handOffs = append(w.handOffs, envelope)
	answer := http.StatusCreated
	if len(w.answers) > 0 {
		answer, w.answers = w.answers[0], w.answers[1:]
	}
	writer.WriteHeader(answer)
}

func (w *world) directory(writer http.ResponseWriter, request *http.Request) {
	w.record("directory "+request.PathValue("domainId"), request)
	w.lock.Lock()
	domain, found := w.domains[request.PathValue("domainId")]
	w.lock.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(writer).Encode(domain)
}

func (w *world) source(writer http.ResponseWriter, request *http.Request) {
	w.record("source "+request.PathValue("producer"), request)
	w.lock.Lock()
	source, found := w.sources[request.PathValue("producer")+" "+request.PathValue("messageId")]
	w.lock.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(writer).Encode(source)
}

func (w *world) messagingEvents(writer http.ResponseWriter, request *http.Request) {
	w.record("push", request)
	envelope := events.Envelope{}
	_ = json.NewDecoder(request.Body).Decode(&envelope)
	w.lock.Lock()
	defer w.lock.Unlock()
	w.pushed = append(w.pushed, envelope)
	writer.WriteHeader(w.eventAnswer)
}

func (w *world) reject(address string) (int, string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if address != w.rejected || !w.rejecting {
		return 0, ""
	}
	return w.rejectCode, w.rejectText
}

func (w *world) unique() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

type sendEnvelope struct {
	EventId    string                    `json:"eventId"`
	Type       string                    `json:"type"`
	DomainId   string                    `json:"domainId"`
	Producer   string                    `json:"producer"`
	OccurredAt string                    `json:"occurredAt"`
	Payload    events.EmailSendRequested `json:"payload"`
}

func (w *world) event(domainId string, from string, rcpt string) sendEnvelope {
	key := "k-" + w.unique()
	messageId := "msg_" + key
	w.lock.Lock()
	w.sources["messaging "+messageId] = MessageSource{MessageId: messageId, DomainId: domainId, Body: "Hello Eve,\n\nsee you **soon**.\n"}
	w.lock.Unlock()
	request := events.EmailSendRequested{
		MessageId: messageId, From: from, Rcpt: rcpt, Role: "to", To: []string{rcpt}, Cc: []string{}, Subject: "Lunch on Friday",
		InReplyTo: "<parent@example.org>", References: []string{"<root@example.org>", "<parent@example.org>"}, AttachmentIds: []string{}, IdempotencyKey: key,
	}
	return sendEnvelope{EventId: "evt_" + w.unique(), Type: "email.send.requested", DomainId: domainId, Producer: "messaging", OccurredAt: Clock{}.Format(time.Now()), Payload: request}
}

func (w *world) send(envelope sendEnvelope) (events.Receipt, error) {
	return w.desk.Receive("Bearer token-"+envelope.DomainId, envelope)
}

func (w *world) mustSend(envelope sendEnvelope) {
	w.t.Helper()
	if _, err := w.send(envelope); err != nil {
		w.t.Fatalf("send: %v", err)
	}
}

func (w *world) run(at time.Time) QueueReport {
	w.t.Helper()
	report, err := w.desk.Run.Run(at)
	if err != nil {
		w.t.Fatalf("run: %v", err)
	}
	return report
}

func (w *world) item(domainId string, eventId string) Item {
	item, _, _ := w.desk.Intake.Queue.FindBy(domainId, "eventId", eventId)
	return item
}

func (w *world) raw(item Item) string {
	path := filepath.Join(w.settings.BlobFolder, filepath.FromSlash(item.RawBlobKey))
	raw, _ := os.ReadFile(path)
	return string(raw)
}

func (w *world) changes() []string {
	w.lock.Lock()
	defer w.lock.Unlock()
	statuses := []string{}
	for _, envelope := range w.pushed {
		change := events.EmailDeliveryChanged{}
		_ = json.Unmarshal(envelope.Payload, &change)
		statuses = append(statuses, envelope.Producer+" "+envelope.DomainId+" "+change.Rcpt+" "+change.Status+" "+change.Detail)
	}
	return statuses
}

func (w *world) recorded() []string {
	w.lock.Lock()
	defer w.lock.Unlock()
	return append([]string{}, w.calls...)
}

func header(raw string, name string) string {
	headers, _, _ := strings.Cut(raw, "\r\n\r\n")
	unfolded := strings.ReplaceAll(strings.ReplaceAll(headers, "\r\n\t", " "), "\r\n ", " ")
	for _, line := range strings.Split(unfolded, "\r\n") {
		field, value, _ := strings.Cut(line, ":")
		if strings.EqualFold(field, name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
