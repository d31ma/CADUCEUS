import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type InboundTestSetup struct {
	Domain   string
	Pusher   *outbox.RecordingPusher
	Names    *CollectionNamesService
	Registry *OutboxCollectionsService
	Store    *OutboxStoreService
	Relay    *OutboxRelayService
	Raw      *InboundRawService
	Inbound  *InboundService
}

func (InboundTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (InboundTestSetup) World() *InboundTestSetup {
	InboundTestSetup{}.RequireFyloRoot()
	documents := &DocumentsService{dataSource: DataSource{}}
	settings := &SettingsService{}
	inboundDocuments := &InboundDocumentService{}
	world := &InboundTestSetup{Pusher: outbox.NewRecordingPusher(), Names: &CollectionNamesService{}}
	world.Domain = "m1-" + world.RandomText() + ".test"
	world.Registry = &OutboxCollectionsService{documents: documents, names: world.Names}
	world.Store = &OutboxStoreService{documents: documents, registry: world.Registry}
	world.Relay = &OutboxRelayService{store: world.Store, settings: settings, pusher: world.Pusher}
	world.Raw = &InboundRawService{documents: documents, names: world.Names, registry: world.Registry, relay: world.Relay}
	staging := &InboundEventService{settings: settings, inboundDocumentService: inboundDocuments, eventPayloadService: &EventPayloadService{}}
	world.Inbound = &InboundService{envelopeCheckService: &EnvelopeCheckService{}, inboundDocumentService: inboundDocuments, inboundRawService: world.Raw, inboundEventService: staging}
	return world
}

func (InboundTestSetup) RequireFyloRoot() {
	snapshot := ApplicationSnapshot{}
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
		Headers:      InboundHeaders{MessageId: "bb-1@outside.test", Subject: "Hello", From: MailboxAddress{Name: "Dan", Address: "dan@outside.test"}},
		AuthResults:  AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass", DmarcPolicy: "none"},
	}
}

func (world *InboundTestSetup) Collection() string {
	return world.Names.InboundRaw(world.Domain)
}

func (world *InboundTestSetup) Stored(key string) InboundRaw {
	found, err := world.Raw.FindByBlobKey(world.Collection(), key)
	world.Must(err)
	return append(found, InboundRaw{})[0]
}

func (world *InboundTestSetup) Fields(stored InboundRaw) outbox.Fields {
	return (&InboundDocumentService{}).OutboxFields(stored)
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

func (world *InboundTestSetup) Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (world *InboundTestSetup) IsNotFound(err error) bool {
	var failure *NotFound
	return errors.As(err, &failure)
}
