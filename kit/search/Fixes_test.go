package search

import (
	"errors"
	"testing"
)

func TestDomainScopeAlwaysWinsOverTheFilter(t *testing.T) {
	scoped := IndexStorage{}.inDomain(map[string]any{"domainId": "other.test", "deleted": false}, "mine.test")
	if scoped["domainId"] != "mine.test" || scoped["deleted"] != false {
		t.Fatalf("got %v", scoped)
	}
	if empty := (IndexStorage{}).inDomain(nil, "mine.test"); empty["domainId"] != "mine.test" {
		t.Fatalf("got %v", empty)
	}
}

func TestAuthorMatchesAnAddressWithoutCaseAndAMemberIdExactly(t *testing.T) {
	document := Document{AuthorAddress: "ana@example.com", AuthorMemberId: "Ana-ID"}
	cases := map[string]bool{"": true, "ANA@example.com": true, "Ana-ID": true, "ana-id": false, "bo@example.com": false}
	for author, want := range cases {
		if got := (Matches{}).authoredBy(document, author); got != want {
			t.Fatalf("author %q: got %v", author, got)
		}
	}
}

func TestJsonCopyRefusesAFieldOfTheWrongType(t *testing.T) {
	target := struct {
		EventId string `json:"eventId"`
	}{}
	err := Json{}.Copy(map[string]any{"eventId": 5}, &target)
	if !errors.As(err, &InvalidRequest{}) {
		t.Fatalf("got %v", err)
	}
}
