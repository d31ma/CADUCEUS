package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func readerIds(count int) []string {
	ids := []string{}
	for index := range count {
		ids = append(ids, fmt.Sprintf("%08d-0000-4000-8000-000000000000", index))
	}
	return ids
}

func TestAChannelEventListsNoReaders(t *testing.T) {
	kept := ReaderLists{}.ForConversation("channel", readerIds(10000))
	encoded, _ := json.Marshal(MessageCreated{ConversationKind: "channel", MemberIds: kept})
	if !strings.Contains(string(encoded), `"memberIds":[]`) {
		t.Fatalf("a channel event listed readers: %d bytes", len(encoded))
	}
}

func TestADirectEventListsItsParticipants(t *testing.T) {
	given := readerIds(3)
	kept := ReaderLists{}.ForConversation("direct", given)
	given[0] = "changed"
	if len(kept) != 3 || kept[0] == "changed" {
		t.Fatalf("got %v", kept)
	}
}

func TestADirectListOfNoMembersIsAnEmptyList(t *testing.T) {
	kept := ReaderLists{}.ForConversation("direct", nil)
	if kept == nil || len(kept) != 0 {
		t.Fatalf("got %v", kept)
	}
}

func TestMentionsKeepTheFirstHundred(t *testing.T) {
	given := readerIds(150)
	kept := ReaderLists{}.Mentions(given)
	if len(kept) != 100 || kept[0] != given[0] || kept[99] != given[99] {
		t.Fatalf("got %d mentions", len(kept))
	}
}

func TestFewerMentionsThanTheLimitAreAllKept(t *testing.T) {
	kept := ReaderLists{}.Mentions(readerIds(2))
	if len(kept) != 2 {
		t.Fatalf("got %v", kept)
	}
}

func TestAnEventIsFromAChannelByKindOrByAnEmptyList(t *testing.T) {
	lists := ReaderLists{}
	if !lists.FromChannel("channel", readerIds(2)) || !lists.FromChannel("", []string{}) || lists.FromChannel("direct", readerIds(1)) || lists.FromChannel("", readerIds(1)) {
		t.Fatal("the channel check is wrong")
	}
}

func TestTheLargestDirectEventStaysSmall(t *testing.T) {
	lists := ReaderLists{}
	event := MessageCreated{ConversationKind: lists.DirectKind(), MemberIds: readerIds(lists.MostDirectMembers()), MentionedMemberIds: lists.Mentions(readerIds(500))}
	encoded, _ := json.Marshal(event)
	if len(encoded) > 16*1024 {
		t.Fatalf("the reader lists take %d bytes", len(encoded))
	}
}
