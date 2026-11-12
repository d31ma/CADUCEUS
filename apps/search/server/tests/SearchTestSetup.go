import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/search"
)

// @TestSetup
type SearchTestSetup struct {
	Domain   string
	Service  search.Service
	readable map[string]map[string]bool
	callers  map[string]oidc.Caller
	failure  error
}

func (SearchTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (SearchTestSetup) World() *SearchTestSetup {
	world := &SearchTestSetup{readable: map[string]map[string]bool{}, callers: map[string]oidc.Caller{}}
	world.Domain = "s1-" + world.RandomText() + ".test"
	world.Service = search.NewService(DataSource{}, world, world)
	return world
}

func (world *SearchTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *SearchTestSetup) Member(domainId string, memberId string, conversationIds ...string) string {
	token := "Bearer " + memberId + "." + domainId
	world.callers[token] = oidc.Caller{DomainId: domainId, MemberId: memberId, Roles: []string{"Member"}}
	world.Allow(token, conversationIds...)
	return token
}

func (world *SearchTestSetup) Allow(token string, conversationIds ...string) {
	allowed := map[string]bool{}
	for _, conversationId := range conversationIds {
		allowed[conversationId] = true
	}
	world.readable[token] = allowed
}

func (world *SearchTestSetup) Identify(authorization string) (oidc.Caller, error) {
	caller, found := world.callers[authorization]
	if !found {
		return oidc.Caller{}, errors.New("unknown token")
	}
	return caller, nil
}

func (world *SearchTestSetup) Readable(authorization string) (map[string]bool, error) {
	return world.readable[authorization], world.failure
}

func (world *SearchTestSetup) Message(conversationId string, body string, createdAt string) events.MessageCreated {
	return events.MessageCreated{
		MessageId:          "msg_" + world.RandomText(),
		ConversationId:     conversationId,
		ConversationKind:   "direct",
		AuthorKind:         "member",
		AuthorMemberId:     "ana-id",
		AuthorAddress:      "Ana@" + world.Domain,
		AuthorDisplayName:  "Ana Lima",
		Body:               body,
		AttachmentIds:      []string{},
		FileNames:          []string{},
		MentionedMemberIds: []string{},
		MemberIds:          []string{},
		CreatedAt:          createdAt,
	}
}

func (world *SearchTestSetup) Envelope(domainId string, eventType string, payload any) events.Envelope {
	encoded, _ := json.Marshal(payload)
	producer, _ := events.Catalog{}.ProducerOf(eventType)
	return events.Envelope{EventID: "evt_" + world.RandomText(), Type: eventType, DomainId: domainId, Producer: producer, OccurredAt: "2026-11-10T10:00:00Z", Payload: encoded}
}

func (world *SearchTestSetup) Receive(envelope events.Envelope) error {
	_, err := world.Service.Receive(oidc.Caller{DomainId: "mesail-services", MemberId: "messaging", Roles: []string{"Service"}}, envelope)
	return err
}

func (world *SearchTestSetup) Created(domainId string, message events.MessageCreated) events.MessageCreated {
	world.Must(world.Receive(world.Envelope(domainId, "message.created", message)))
	return message
}

func (world *SearchTestSetup) Edit(domainId string, message events.MessageCreated, body string) {
	edited := events.MessageEdited{MessageId: message.MessageId, ConversationId: message.ConversationId, Body: body, MemberIds: []string{}, EditedAt: "2026-11-10T12:00:00Z"}
	world.Must(world.Receive(world.Envelope(domainId, "message.edited", edited)))
}

func (world *SearchTestSetup) Delete(domainId string, message events.MessageCreated) {
	deleted := events.MessageDeleted{MessageId: message.MessageId, ConversationId: message.ConversationId, MemberIds: []string{}, DeletedAt: "2026-11-10T13:00:00Z"}
	world.Must(world.Receive(world.Envelope(domainId, "message.deleted", deleted)))
}

func (world *SearchTestSetup) Search(token string, values url.Values) search.Page {
	page, err := world.Service.Search(token, values)
	world.Must(err)
	return page
}

func (world *SearchTestSetup) Ids(page search.Page) []string {
	ids := []string{}
	for _, hit := range page.Items {
		ids = append(ids, hit.MessageId)
	}
	return ids
}

func (world *SearchTestSetup) Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (world *SearchTestSetup) SignedIn(domainId string, subject string, realmRole string, party string) string {
	keys, _ := os.MkdirTemp("", "s1-keys-")
	os.Setenv("TMPDIR", keys)
	started, _ := oidc.SigningRealm{}.StartedMesail()
	realm := started.ForDomain(domainId)
	snapshot := struct {
		Yon struct {
			Oidc   oidc.Settings   `json:"oidc"`
			Search search.Settings `json:"search"`
		} `json:"yon"`
	}{}
	snapshot.Yon.Oidc = realm.Settings
	snapshot.Yon.Search = search.Settings{MessagingUrl: "http://127.0.0.1:9", ChannelsUrl: "http://127.0.0.1:9"}
	text, _ := json.Marshal(snapshot)
	os.Setenv("YON_APPLICATION_JSON", string(text))
	realm.Publish(time.Now())
	request := oidc.TokenRequest{Subject: subject, Email: subject + "@" + domainId, Name: subject, Username: subject, RealmRoles: []string{realmRole}, Audience: "mesail-api", AuthorizedParty: party, ExpiresAt: time.Now().Unix() + 60}
	token, _ := realm.Token(request)
	return "Bearer " + token
}
