import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type ChannelReadersTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelReadersTest) TestAChannelEventWithoutReadersReachesEveryChannelMember() {
	domainId := "k5-" + strings.ToLower(rand.Text())[:12] + ".test"
	pushes := make(chan string, 100)
	desk := test.desk(pushes)
	test.handled(desk, domainId, "channel.member.added", events.ChannelMemberAdded{ChannelId: "chn_1", ConversationId: "cnv_1", MemberId: "ana"})
	test.seeded(domainId, 110)
	test.connected(desk, domainId, map[string]string{"c-ana": "ana", "c-105": "m00105", "c-dan": "dan"})
	test.handled(desk, domainId, "message.created", events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "channel", AuthorMemberId: "ana", MemberIds: []string{}})
	Assert.Equals([]string{"c-105", "c-ana"}, test.drained(pushes))
	test.handled(desk, domainId, "channel.member.removed", events.ChannelMemberRemoved{ChannelId: "chn_1", ConversationId: "cnv_1", MemberId: "m00105"})
	test.handled(desk, domainId, "reaction.changed", events.ReactionChanged{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "ana", Emoji: "+1", Added: true, MemberIds: []string{}})
	Assert.Equals([]string{"c-ana"}, test.drained(pushes))
	members, err := desk.Members.Channel(domainId, "cnv_1")
	Assert.IsTrue(err == nil, fmt.Sprintf("members failed: %v", err))
	Assert.Equals(109, len(members))
}

func (test *ChannelReadersTest) desk(pushes chan string) realtime.EventDesk {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		pushes <- strings.TrimPrefix(request.URL.Path, "/@connections/")
		writer.WriteHeader(http.StatusOK)
	}))
	storage := StorageUtils{}.From(DataSource{})
	connections := realtime.ConnectionBook{Storage: storage, Now: time.Now}
	gateway := realtime.HttpConnectionGateway{BaseUrl: server.URL, Secret: strings.Repeat("g", 32), Client: server.Client()}
	return realtime.EventDesk{Members: realtime.MemberBook{Storage: storage}, FanOut: realtime.FanOut{Connections: connections, Gateway: gateway}}
}

func (test *ChannelReadersTest) seeded(domainId string, memberCount int) {
	collection := collections.CollectionName{}.ForDomain("channel-members", domainId)
	for index := 1; index < memberCount; index++ {
		row := realtime.ChannelMember{DomainId: domainId, ConversationId: "cnv_1", MemberId: fmt.Sprintf("m%05d", index)}
		_, err := DataSource{}.Insert(collection, row)
		Assert.IsTrue(err == nil, fmt.Sprintf("seed failed: %v", err))
	}
}

func (test *ChannelReadersTest) connected(desk realtime.EventDesk, domainId string, members map[string]string) {
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	for connectionId, memberId := range members {
		err := desk.FanOut.Connections.Add(realtime.Connection{ConnectionId: connectionId, DomainId: domainId, MemberId: memberId, Kind: "socket", ExpiresAt: expires})
		Assert.IsTrue(err == nil, fmt.Sprintf("connect failed: %v", err))
	}
}

func (test *ChannelReadersTest) handled(desk realtime.EventDesk, domainId string, eventType string, payload any) {
	raw, _ := json.Marshal(payload)
	producer, _ := events.Catalog{}.ProducerOf(eventType)
	envelope := events.Envelope{EventID: "evt_" + strings.ToLower(rand.Text())[:12], Type: eventType, DomainId: domainId, Producer: producer, OccurredAt: time.Now().UTC().Format(time.RFC3339), Payload: raw}
	err := desk.Handle(envelope)
	Assert.IsTrue(err == nil, fmt.Sprintf("%s failed: %v", eventType, err))
}

func (test *ChannelReadersTest) drained(pushes chan string) []string {
	received := []string{}
	for len(pushes) > 0 {
		received = append(received, <-pushes)
	}
	return realtime.MemberSet{}.Sorted(received)
}
