import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type InboundTestSetup struct {
	Domain    string
	Pusher    *outbox.RecordingPusher
	Workflows *WorkflowService
	Inbound   *InboundController
	Outbox    *OutboxController
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
