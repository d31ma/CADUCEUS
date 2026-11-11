package realtime

import (
	"reflect"
	"testing"
)

func TestMembersAnswersAnEmptyListForAnUnknownConversation(t *testing.T) {
	members, err := MemberBook{Storage: newMemoryStorage().storage()}.Members("example.com", "cnv_1")
	if err != nil || members == nil || len(members) != 0 {
		t.Fatalf("got %v %v", members, err)
	}
}

func TestReplaceWritesOnlyWhenTheSetChanged(t *testing.T) {
	memory := newMemoryStorage()
	book := MemberBook{Storage: memory.storage()}
	first := book.Replace("example.com", "cnv_1", []string{"bo", "ana", "bo"})
	writes := memory.writes
	second := book.Replace("example.com", "cnv_1", []string{"ana", "bo"})
	unchangedWrites := memory.writes
	third := book.Replace("example.com", "cnv_1", []string{"ana"})
	members, _ := book.Members("example.com", "cnv_1")
	if first != nil || second != nil || third != nil || writes != 1 || unchangedWrites != 1 || memory.writes != 2 || !reflect.DeepEqual(members, []string{"ana"}) {
		t.Fatalf("writes %d %d %d, members %v", writes, unchangedWrites, memory.writes, members)
	}
}

func TestAddIsIdempotent(t *testing.T) {
	memory := newMemoryStorage()
	book := MemberBook{Storage: memory.storage()}
	book.Add("example.com", "cnv_1", "ana")
	book.Add("example.com", "cnv_1", "ana")
	book.Add("example.com", "cnv_1", "bo")
	members, _ := book.Members("example.com", "cnv_1")
	if memory.writes != 2 || !reflect.DeepEqual(members, []string{"ana", "bo"}) {
		t.Fatalf("writes %d, members %v", memory.writes, members)
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	memory := newMemoryStorage()
	book := MemberBook{Storage: memory.storage()}
	book.Replace("example.com", "cnv_1", []string{"ana", "bo"})
	book.Remove("example.com", "cnv_1", "bo")
	book.Remove("example.com", "cnv_1", "bo")
	book.Remove("example.com", "cnv_2", "bo")
	members, _ := book.Members("example.com", "cnv_1")
	if memory.writes != 2 || !reflect.DeepEqual(members, []string{"ana"}) {
		t.Fatalf("writes %d, members %v", memory.writes, members)
	}
}

func TestMembersAreKeptPerDomain(t *testing.T) {
	book := MemberBook{Storage: newMemoryStorage().storage()}
	book.Add("example.com", "cnv_1", "ana")
	members, _ := book.Members("example.org", "cnv_1")
	if len(members) != 0 {
		t.Fatalf("got %v", members)
	}
}

func TestMemberBookPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	memory.failAll = true
	book := MemberBook{Storage: memory.storage()}
	if book.Replace("example.com", "cnv_1", []string{"ana"}) == nil || book.Add("example.com", "cnv_1", "ana") == nil || book.Remove("example.com", "cnv_1", "ana") == nil {
		t.Fatal("a failing storage was not reported")
	}
}
