import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type InboundTestSetup struct {
	Domain      string
	Sender      string
	Retired     string
	Other       string
	Pusher      *outbox.RecordingPusher
	Workflows   *WorkflowService
	Inbound     *InboundController
	Outbox      *OutboxController
	server      *httptest.Server
	realm       oidc.SigningRealm
	lock        sync.Mutex
	asked       []string
	savedJson   string
	savedTemp   string
	tokenFolder string
	blobFolder  string
}

func (InboundTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (InboundTestSetup) World() *InboundTestSetup {
	InboundTestSetup{}.RequireFyloRoot()
	world := &InboundTestSetup{Pusher: outbox.NewRecordingPusher()}
	world.Domain = "m1-" + world.RandomText() + ".test"
	world.Workflows = &WorkflowService{dataSource: DataSource{}, pusher: world.Pusher}
	world.Inbound = &InboundController{workflows: world.Workflows}
	world.Outbox = &OutboxController{workflows: world.Workflows}
	return world
}

func (InboundTestSetup) RequireFyloRoot() {
	snapshot := mailin.ApplicationSnapshot{}
	json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &snapshot)
	settings := map[string]json.RawMessage{}
	json.Unmarshal(snapshot.Yon, &settings)
	if _, found := settings["datasource"]; !found {
		panic("mail-in storage tests need a FYLO root (SLICES rule 11): copy apps/mail-in/.env.example to apps/mail-in/.env, set YON_DATASOURCE_URL to fylo://<an existing folder>, and create that folder")
	}
}

func (world *InboundTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *InboundTestSetup) FreshKey() string {
	return world.Domain + "/inbound-" + world.RandomText()
}

func (world *InboundTestSetup) StoredKey() string {
	if world.server == nil {
		world.UseServices()
	}
	key := world.FreshKey()
	world.WriteBlob(key, world.TextMessage())
	return key
}

func (world *InboundTestSetup) Envelope(key string) InboundEnvelope {
	return InboundEnvelope{
		EnvelopeFrom: "dan@outside.test",
		Recipients:   []string{"ana@" + world.Domain},
		RawBlobKey:   key,
		Size:         64,
		Headers:      InboundHeaders{MessageId: "bb-1@outside.test", Subject: "Hello", From: MailboxAddress{Name: "Dan", Address: "dan@outside.test"}, To: []MailboxAddress{{Name: "Ana", Address: "ana@" + world.Domain}}},
		AuthResults:  AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass", DmarcPolicy: "none"},
	}
}

func (world *InboundTestSetup) Collection() string {
	return collections.CollectionName{}.ForDomain("inbound-raw", world.Domain)
}

func (world *InboundTestSetup) Stored(key string) mailin.InboundRaw {
	found, err := world.Workflows.Inbox().FindByBlobKey(world.Collection(), key)
	world.Must(err)
	return append(found, mailin.InboundRaw{})[0]
}

func (world *InboundTestSetup) PushedToMessaging() []events.Envelope {
	matching := []events.Envelope{}
	for _, envelope := range world.Pusher.PushedTo("messaging") {
		matching = append(matching, world.ifThisDomain(envelope)...)
	}
	return matching
}

func (world *InboundTestSetup) ifThisDomain(envelope events.Envelope) []events.Envelope {
	if envelope.DomainId == world.Domain {
		return []events.Envelope{envelope}
	}
	return []events.Envelope{}
}

func (world *InboundTestSetup) DeadFor(documentId string) []DeadEvent {
	dead, err := world.Outbox.GetDead()
	world.Must(err)
	kept := []DeadEvent{}
	for _, event := range dead {
		kept = append(kept, world.ifDocument(event, documentId)...)
	}
	return kept
}

func (world *InboundTestSetup) ifDocument(event DeadEvent, documentId string) []DeadEvent {
	if event.DocumentId == documentId {
		return []DeadEvent{event}
	}
	return []DeadEvent{}
}

func (world *InboundTestSetup) Cleanup() {
	world.Forget(world.Collection())
	DataSource{}.DropCollection(mailin.ContentShelf{}.Collection(world.Domain))
	world.stopServices()
}

func (world *InboundTestSetup) UseServices() {
	world.Sender = "m1s-" + world.RandomText() + ".test"
	world.Retired = "m1r-" + world.RandomText() + ".test"
	world.Other = "m1o-" + world.RandomText() + ".test"
	world.savedJson, world.savedTemp = os.Getenv("YON_APPLICATION_JSON"), os.Getenv("TMPDIR")
	world.tokenFolder, _ = os.MkdirTemp("", "mail-in-realms-")
	os.Setenv("TMPDIR", world.tokenFolder)
	world.realm, _ = oidc.SigningRealm{}.StartedMesail()
	world.realm.Publish(time.Now())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/directory/domains/{domainId}", world.directory)
	mux.HandleFunc("POST /api/v1/identity/service-token", world.identityServiceToken)
	world.server = httptest.NewServer(mux)
	world.writeSettings()
}

func (world *InboundTestSetup) writeSettings() {
	application := map[string]map[string]any{}
	world.Must(json.Unmarshal([]byte(world.savedJson), &application))
	identity := world.realm.Settings
	identity.ServiceTokens = world.server.URL + "/api/v1/identity/service-token"
	settings := mailin.SettingsReader{Environment: os.Getenv}.MailIn()
	settings.Directory = world.server.URL
	world.blobFolder = cmp.Or(settings.Blobs.Folder, world.tokenFolder)
	settings.Blobs = mailin.BlobSettings{Adapter: "local", Folder: world.blobFolder, Link: mailin.LinkSettings{Base: "https://mesail.test/blobs", Seconds: 300, Secret: cmp.Or(settings.Blobs.Link.Secret, "fixture-link-value")}}
	application["yon"]["oidc"] = identity
	application["yon"]["service"] = map[string]any{"client": map[string]string{"secret": "fixture-client-value"}}
	application["yon"]["mailin"] = settings
	text, _ := json.Marshal(application)
	os.Setenv("YON_APPLICATION_JSON", string(text))
}

func (world *InboundTestSetup) stopServices() {
	if world.server == nil {
		return
	}
	world.server.Close()
	os.Setenv("YON_APPLICATION_JSON", world.savedJson)
	os.Setenv("TMPDIR", world.savedTemp)
	for _, domainId := range []string{world.Domain, world.Sender, world.Retired} {
		os.RemoveAll(filepath.Join(world.blobFolder, domainId))
	}
	os.RemoveAll(world.tokenFolder)
}

func (world *InboundTestSetup) identityServiceToken(writer http.ResponseWriter, request *http.Request) {
	asked := oidc.ServiceTokenRequest{}
	json.NewDecoder(request.Body).Decode(&asked)
	json.NewEncoder(writer).Encode(oidc.ServiceTokenAnswer{AccessToken: world.serviceToken(asked.DomainId), ExpiresIn: 60})
}

func (world *InboundTestSetup) directory(writer http.ResponseWriter, request *http.Request) {
	caller, _ := oidc.Configured().Identify(request.Header.Get("Authorization"))
	domainId := request.PathValue("domainId")
	world.lock.Lock()
	world.asked = append(world.asked, domainId+" "+caller.DomainId)
	world.lock.Unlock()
	states := map[string]string{world.Sender: "active", world.Retired: "disabled"}
	if states[domainId] == "" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(writer).Encode(mailin.Domain{DomainId: domainId, Name: domainId, State: states[domainId]})
}

func (world *InboundTestSetup) serviceToken(domainId string) string {
	token, _ := world.realm.ForDomain(domainId).Token(oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
	return token
}

func (world *InboundTestSetup) Bearer(domainId string) string {
	return "Bearer " + world.serviceToken(domainId)
}

func (world *InboundTestSetup) Asked() []string {
	world.lock.Lock()
	defer world.lock.Unlock()
	return append([]string{}, world.asked...)
}

func (world *InboundTestSetup) Blobs() blobs.Store {
	settings := mailin.SettingsReader{Environment: os.Getenv}.MailIn()
	return mailin.BlobStoreFactory{Environment: os.Getenv}.Store(settings.Blobs)
}

func (world *InboundTestSetup) WriteBlob(path string, content string) {
	world.Must(world.Blobs().Write(mailin.BlobKeys{}.FromPath(path), strings.NewReader(content), int64(len(content))))
}

func (world *InboundTestSetup) ReadBlob(path string) string {
	reader, err := world.Blobs().Read(mailin.BlobKeys{}.FromPath(path))
	world.Must(err)
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	return string(content)
}

func (world *InboundTestSetup) DeleteBlob(path string) {
	world.Must(world.Blobs().Delete(mailin.BlobKeys{}.FromPath(path)))
}

func (world *InboundTestSetup) HandOff(sender string) InboundEnvelope {
	envelope := world.Envelope(sender + "/inbound-" + world.RandomText())
	envelope.EnvelopeFrom = "bob@" + sender
	envelope.Headers.From = MailboxAddress{Name: "Bob", Address: "bob@" + sender}
	envelope.AuthResults = AuthResults{Handoff: true}
	return envelope
}

func (world *InboundTestSetup) Accepted(raw string) string {
	key := world.FreshKey()
	world.WriteBlob(key, raw)
	accepted, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	return accepted.InboundId
}

func (world *InboundTestSetup) IsUnauthenticated(err error) bool {
	var failure *Unauthenticated
	return errors.As(err, &failure)
}

func (world *InboundTestSetup) InvalidField(err error) string {
	failure := &InvalidEnvelope{}
	errors.As(err, &failure)
	return failure.Field
}

func (world *InboundTestSetup) HtmlMessage() string {
	return strings.Join([]string{
		"From: Dan <dan@outside.test>", "Subject: Plans", "MIME-Version: 1.0", `Content-Type: multipart/mixed; boundary="outer"`, "",
		"--outer", `Content-Type: multipart/related; boundary="inner"`, "",
		"--inner", "Content-Type: text/html; charset=utf-8", "",
		`<p>Hello <b>Ana</b></p><img src="cid:logo@outside.test" alt="Logo">`,
		"--inner", "Content-Type: image/png", "Content-ID: <logo@outside.test>", `Content-Disposition: inline; filename="logo.png"`, "Content-Transfer-Encoding: base64", "",
		"iVBORw0KGgo=",
		"--inner--",
		"--outer", "Content-Type: application/pdf", `Content-Disposition: attachment; filename="plan.pdf"`, "Content-Transfer-Encoding: base64", "",
		"JVBERi0xLjQK",
		"--outer--", "",
	}, "\r\n")
}

func (world *InboundTestSetup) LongTextMessage(piece string, count int) string {
	header := "From: Dan <dan@outside.test>\r\nSubject: Long\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n"
	return header + strings.Repeat(piece, count) + "\r\n"
}

func (world *InboundTestSetup) StagedThenRelayed(raw string) events.EmailReceived {
	world.UseServices()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	world.WriteBlob(key, raw)
	_, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	payload := events.EmailReceived{}
	_, staged := outbox.Published{}.One(world.Stored(key).OutboxFields(), events.Types{}.EmailReceived(), &payload)
	world.Must(staged)
	world.Pusher.Recover("messaging")
	_, relayed := world.Outbox.PostRelay()
	world.Must(relayed)
	Assert.Equals(0, len(world.Stored(key).PendingEvents))
	Assert.Equals(1, len(world.PushedToMessaging()))
	return payload
}

func (world *InboundTestSetup) TextMessage() string {
	return "From: Dan <dan@outside.test>\r\nSubject: Lunch\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nLunch at noon?\r\n"
}

func (world *InboundTestSetup) ImageOf(content InboundContent) ImageRef {
	images := []ImageRef{}
	for _, block := range content.Blocks {
		images = append(images, world.imageIn(block)...)
	}
	return append(images, ImageRef{})[0]
}

func (world *InboundTestSetup) imageIn(block Block) []ImageRef {
	if block.Image == nil {
		return []ImageRef{}
	}
	return []ImageRef{*block.Image}
}

func (world *InboundTestSetup) Forget(collection string) {
	registry := mailin.CollectionRegistry{}.Name()
	for _, record := range world.RegistryRecords(collection) {
		_, err := DataSource{}.DeleteById(registry, record.Id)
		world.Must(err)
	}
	DataSource{}.DropCollection(collection)
}

func (world *InboundTestSetup) RegistryRecords(collection string) []mailin.CollectionRecord {
	window, err := DataSource{}.Scroll(mailin.CollectionRegistry{}.Name(), map[string]any{"name": collection}, "", 8)
	world.Must(err)
	records := []mailin.CollectionRecord{}
	if len(window.Content) > 0 {
		world.Must(json.Unmarshal(window.Content, &records))
	}
	return records
}

func (world *InboundTestSetup) Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (world *InboundTestSetup) IsNotFound(err error) bool {
	var failure *NotFound
	return errors.As(err, &failure)
}

func (world *InboundTestSetup) IsInvalid(err error) bool {
	var failure *InvalidEnvelope
	return errors.As(err, &failure)
}
