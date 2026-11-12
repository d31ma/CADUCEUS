package realtime

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"mesail.dev/packages/events"
)

func channelScene(t *testing.T) (EventDesk, *recordingGateway) {
	t.Helper()
	gateway := newRecordingGateway()
	desk := presenceEventDesk(newMemoryStorage(), gateway)
	for _, memberId := range []string{"ana", "bo"} {
		handled(t, desk, "channel.member.added", events.ChannelMemberAdded{ChannelId: "chn_1", ConversationId: "cnv_1", MemberId: memberId})
	}
	for index, memberId := range []string{"ana", "bo", "dan"} {
		if err := desk.FanOut.Connections.Add(testConnection("example.com", memberId, fmt.Sprintf("c%d", index+1))); err != nil {
			t.Fatal(err)
		}
	}
	return desk, gateway
}

func received(gateway *recordingGateway, eventType string) []int {
	return []int{len(gateway.eventsOfType("c1", eventType)), len(gateway.eventsOfType("c2", eventType)), len(gateway.eventsOfType("c3", eventType))}
}

func TestAChannelMessageReachesTheChannelMembers(t *testing.T) {
	desk, gateway := channelScene(t)
	handled(t, desk, "message.created", events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "channel", AuthorMemberId: "ana", MemberIds: []string{}})
	if got := received(gateway, "message.created"); !reflect.DeepEqual(got, []int{1, 1, 0}) {
		t.Fatalf("got %v", got)
	}
}

func TestChannelEditsDeletesAndReactionsReachTheChannelMembers(t *testing.T) {
	desk, gateway := channelScene(t)
	handled(t, desk, "message.edited", events.MessageEdited{MessageId: "msg_1", ConversationId: "cnv_1", MemberIds: []string{}})
	handled(t, desk, "message.deleted", events.MessageDeleted{MessageId: "msg_1", ConversationId: "cnv_1", MemberIds: []string{}})
	handled(t, desk, "reaction.changed", events.ReactionChanged{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "bo", Emoji: "+1", Added: true, MemberIds: []string{}})
	got := fmt.Sprint(received(gateway, "message.edited"), received(gateway, "message.deleted"), received(gateway, "reaction.changed"))
	if got != "[1 1 0] [1 1 0] [1 1 0]" {
		t.Fatalf("got %s", got)
	}
}

func TestAnOldChannelEventWithReadersUsesTheChannelCopy(t *testing.T) {
	desk, gateway := channelScene(t)
	handled(t, desk, "message.created", events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "channel", AuthorMemberId: "ana", MemberIds: []string{"dan"}})
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if got := received(gateway, "message.created"); !reflect.DeepEqual(got, []int{1, 1, 0}) || !reflect.DeepEqual(members, []string{"ana", "bo"}) {
		t.Fatalf("got %v, members %v", got, members)
	}
}

func TestAChannelMessageKeepsTheChannelCopy(t *testing.T) {
	desk, _ := channelScene(t)
	handled(t, desk, "message.created", events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "channel", AuthorMemberId: "ana", MemberIds: []string{}})
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if !reflect.DeepEqual(members, []string{"ana", "bo"}) {
		t.Fatalf("got %v", members)
	}
}

func TestADirectMessageReachesItsListedReaders(t *testing.T) {
	desk, gateway := channelScene(t)
	handled(t, desk, "message.created", events.MessageCreated{MessageId: "msg_2", ConversationId: "cnv_2", ConversationKind: "direct", AuthorMemberId: "ana", MemberIds: []string{"ana", "dan"}})
	members, _ := desk.Members.Members("example.com", "cnv_2")
	if got := received(gateway, "message.created"); !reflect.DeepEqual(got, []int{1, 0, 1}) || !reflect.DeepEqual(members, []string{"ana", "dan"}) {
		t.Fatalf("got %v, members %v", got, members)
	}
}

func TestAChannelOfManyMembersIsReadAcrossPages(t *testing.T) {
	book := ChannelBook{Storage: newMemoryStorage().storage()}
	for index := range 250 {
		if err := book.Add("example.com", "cnv_1", fmt.Sprintf("m%03d", index)); err != nil {
			t.Fatal(err)
		}
	}
	members, err := book.Members("example.com", "cnv_1")
	if err != nil || len(members) != 250 || members[249] != "m249" {
		t.Fatalf("got %d members, %v", len(members), err)
	}
}

func TestMembersPreferTheChannelCopy(t *testing.T) {
	book := MemberBook{Storage: newMemoryStorage().storage()}
	book.Replace("example.com", "cnv_1", []string{"old"})
	book.Add("example.com", "cnv_1", "bo")
	book.Replace("example.com", "cnv_2", []string{"ana"})
	channel, _ := book.Members("example.com", "cnv_1")
	direct, _ := book.Members("example.com", "cnv_2")
	if !reflect.DeepEqual(channel, []string{"bo"}) || !reflect.DeepEqual(direct, []string{"ana"}) {
		t.Fatalf("got %v and %v", channel, direct)
	}
}

func TestTheChannelCopyPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	memory.failAll = true
	book := MemberBook{Storage: memory.storage()}
	_, membersError := book.Members("example.com", "cnv_1")
	_, channelError := book.Channel("example.com", "cnv_1")
	if membersError == nil || channelError == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestAChannelDeliveryPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	desk := presenceEventDesk(memory, newRecordingGateway())
	memory.failAll = true
	err := desk.deliver("example.com", Delivery{Event: Event{Type: "message.created"}, ChannelConversationId: "cnv_1"})
	if err == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestEveryPassesAScrollFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	storage := Storage{Documents: memory, Scroll: func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{}, errors.New("scroll is down")
	}}
	_, err := ChannelBook{Storage: storage}.Members("example.com", "cnv_1")
	if err == nil {
		t.Fatal("a failing scroll was not reported")
	}
}

func TestEveryRefusesRowsOfTheWrongShape(t *testing.T) {
	memory := newMemoryStorage()
	storage := Storage{Documents: memory, Scroll: func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{Content: []byte(`[{"memberId":5}]`)}, nil
	}}
	_, err := ChannelBook{Storage: storage}.Members("example.com", "cnv_1")
	if err == nil {
		t.Fatal("a row of the wrong shape was not reported")
	}
}
