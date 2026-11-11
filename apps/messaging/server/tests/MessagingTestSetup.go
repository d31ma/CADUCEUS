import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
)

// @TestSetup
type MessagingTestSetup struct {
	Domain        string
	Fakes         *fakes.FakeWorld
	Core          *CoreService
	Service       *MessagingService
	Home          oidc.SigningRealm
	Away          oidc.SigningRealm
	Conversations *ConversationsController
	Messages      *MessagesController
	Sources       *MessageSourceController
	Attachments   *AttachmentsController
	Events        *EventsController
}

func (MessagingTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (MessagingTestSetup) World() *MessagingTestSetup {
	world := &MessagingTestSetup{Fakes: fakes.NewFakeWorld()}
	world.Domain = "m3-" + world.RandomText() + ".test"
	world.Home, world.Away = world.SignedRealms()
	world.Core = &CoreService{fixedStores: &world.Fakes.Stores, fixedPorts: &world.Fakes.Ports}
	world.Service = &MessagingService{core: world.Core, failures: &FailureService{}}
	callers := &CallerService{}
	requests := &RequestService{}
	wire := &WireService{}
	world.Conversations = &ConversationsController{callers: callers, requests: requests, messaging: world.Service, wire: wire}
	world.Messages = &MessagesController{callers: callers, messaging: world.Service, wire: wire}
	world.Attachments = &AttachmentsController{callers: callers, messaging: world.Service}
	world.Sources = &MessageSourceController{callers: callers, messaging: world.Service, wire: wire}
	world.Events = &EventsController{callers: callers, events: world.EventsService(), wire: wire}
	world.AddMember("ana")
	world.AddMember("bo")
	return world
}

func (world *MessagingTestSetup) SignedRealms() (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "messaging-test-")
	world.Must(err)
	os.Setenv("TMPDIR", folder)
	home, err := oidc.SigningRealm{}.Started(world.Domain)
	world.Must(err)
	away, err := oidc.SigningRealm{}.Started(world.OtherDomain())
	world.Must(err)
	world.Must(home.Publish(time.Now()))
	world.Must(away.Publish(time.Now()))
	document, err := home.ApplicationJson()
	world.Must(err)
	os.Setenv("YON_APPLICATION_JSON", document)
	return home, away
}

func (world *MessagingTestSetup) OtherDomain() string {
	return "other-" + world.Domain
}

func (world *MessagingTestSetup) AddMember(memberId string) {
	address := world.Address(memberId)
	world.Fakes.Directory.Member(address, memberId, memberId)
	member := core.Member{DomainId: world.Domain, MemberId: memberId, LocalPart: memberId, DisplayName: memberId, PrimaryAddress: address}
	world.Must(world.Fakes.Stores.Members.Save(member))
}

func (world *MessagingTestSetup) Address(memberId string) string {
	return memberId + "@" + world.Domain
}

func (world *MessagingTestSetup) Bearer(realm oidc.SigningRealm, memberId string, realmRole string) string {
	request := oidc.TokenRequest{Subject: memberId, Name: memberId, Email: memberId + "@" + realm.Name, RealmRoles: []string{realmRole}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60}
	token, err := realm.Token(request)
	world.Must(err)
	return "Bearer " + token
}

func (world *MessagingTestSetup) MemberBearer(memberId string) string {
	return world.Bearer(world.Home, memberId, "mesail-member")
}

func (world *MessagingTestSetup) StrangerBearer(memberId string) string {
	return world.Bearer(world.Away, memberId, "mesail-member")
}

func (world *MessagingTestSetup) ServiceBearer(realm oidc.SigningRealm) string {
	request := oidc.TokenRequest{Subject: "service-account", Name: "service", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60}
	token, err := realm.Token(request)
	world.Must(err)
	return "Bearer " + token
}

func (world *MessagingTestSetup) Member(memberId string) oidc.Caller {
	return oidc.Caller{DomainId: world.Domain, MemberId: memberId, DisplayName: memberId, Email: world.Address(memberId), Roles: []string{"Member"}}
}

func (world *MessagingTestSetup) ChatTo(memberId string, clientMessageId string, body string) SendResult {
	request := SendRequest{ClientMessageId: clientMessageId, Body: body, Recipients: []RecipientChoice{{Address: world.Address(memberId), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	world.Must(err)
	return result
}

func (world *MessagingTestSetup) Reply(conversationId string, clientMessageId string, body string) SendResult {
	request := SendRequest{ClientMessageId: clientMessageId, ConversationId: conversationId, Body: body}
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	world.Must(err)
	return result
}

func (world *MessagingTestSetup) WithAttachment(memberId string, attachmentId string) SendResult {
	world.Fakes.Attachments.Ready(world.Domain, attachmentId, attachmentId+".txt", 100)
	request := SendRequest{ClientMessageId: "c-" + attachmentId, Body: "file", AttachmentIds: []string{attachmentId}, Recipients: []RecipientChoice{{Address: world.Address(memberId), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	world.Must(err)
	return result
}

func (world *MessagingTestSetup) Request(target string, authorization string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", authorization)
	return request
}

func (world *MessagingTestSetup) MessagesOf(conversationId string, authorization string) MessagePage {
	page, err := world.Conversations.GetMessages(conversationId, world.Request("/api/v1/messaging/conversations/"+conversationId+"/messages", authorization))
	world.Must(err)
	return page
}

func (world *MessagingTestSetup) Channels(statuses []DeliveryStatus) []string {
	channels := []string{}
	for _, status := range statuses {
		channels = append(channels, status.Recipient+"/"+status.Channel)
	}
	return channels
}

func (world *MessagingTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *MessagingTestSetup) Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (world *MessagingTestSetup) IsUnauthenticated(err error) bool {
	var failure *Unauthenticated
	return errors.As(err, &failure)
}

func (world *MessagingTestSetup) EventsService() *EventsService {
	return &EventsService{core: world.Core, storage: &StorageService{settings: &SettingsService{}}, failures: &FailureService{}}
}

func (world *MessagingTestSetup) IsForbidden(err error) bool {
	var failure *Forbidden
	return errors.As(err, &failure)
}

func (world *MessagingTestSetup) IsNotFound(err error) bool {
	var failure *NotFound
	return errors.As(err, &failure)
}

func (world *MessagingTestSetup) IsUnavailable(err error) bool {
	var failure *Unavailable
	return errors.As(err, &failure)
}

func (world *MessagingTestSetup) IsTooLarge(err error) bool {
	var failure *TooLarge
	return errors.As(err, &failure)
}

func (world *MessagingTestSetup) InvalidField(err error) string {
	failure := &InvalidRequest{}
	errors.As(err, &failure)
	return failure.Field
}

func (world *MessagingTestSetup) Changed(err error) *ChangedRecipients {
	failure := &ChangedRecipients{}
	errors.As(err, &failure)
	return failure
}

func (world *MessagingTestSetup) NotAllowed(err error) *ChatNotAllowed {
	failure := &ChatNotAllowed{}
	errors.As(err, &failure)
	return failure
}
