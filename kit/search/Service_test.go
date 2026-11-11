package search

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

func TestSearchKeepsEachDomainToItsOwnMessages(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_a", "cnv_1", "quarterly budget review", "2026-11-05T10:00:00Z"))
	f.create(domainB, message("msg_b", "cnv_1", "quarterly budget review", "2026-11-05T10:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", "budget"), "msg_a")
	sameIds(t, f.searchIds(tokenB, "q", "budget"), "msg_b")
	sameIds(t, f.searchIds(tokenA), "msg_a")
	sameIds(t, f.searchIds(tokenB), "msg_b")
}

func TestSearchKeepsDomainsApartForTheSameMessageId(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_same", "cnv_1", "alpha words", "2026-11-05T10:00:00Z"))
	f.create(domainB, message("msg_same", "cnv_1", "bravo words", "2026-11-05T10:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", "bravo"))
	sameIds(t, f.searchIds(tokenB, "q", "alpha"))
	sameIds(t, f.searchIds(tokenB, "q", "bravo"), "msg_same")
}

func TestMatchesKeepDropsADocumentOfAnotherDomain(t *testing.T) {
	documents := []Document{
		{DomainId: domainA, MessageId: "msg_a", ConversationId: "cnv_1", CreatedAt: "2026-11-05T10:00:00.000000Z"},
		{DomainId: domainB, MessageId: "msg_b", ConversationId: "cnv_1", CreatedAt: "2026-11-05T10:00:00.000000Z"},
	}
	kept := Matches{}.Keep(documents, domainA, Filter{Limit: 20}, map[string]bool{"cnv_1": true})
	if len(kept) != 1 || kept[0].MessageId != "msg_a" {
		t.Fatalf("got %+v", kept)
	}
}

func TestSearchHidesConversationsTheReaderDoesNotList(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_hidden", "cnv_secret", "budget", "2026-11-05T10:00:00Z"))
	f.create(domainA, message("msg_open", "cnv_1", "budget", "2026-11-05T09:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", "budget"), "msg_open")
}

func TestSearchIsUnavailableWhenMembershipFails(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "budget", "2026-11-05T10:00:00Z"))
	f.reader.err = errors.New("messaging is down")
	page, err := f.service.Search(tokenA, query("q", "budget"))
	if !errors.As(err, &Unavailable{}) || page.Items != nil || page.NextCursor != "" {
		t.Fatalf("got %+v %v", page, err)
	}
	if !errors.Is(err, f.reader.err) {
		t.Fatalf("the cause is lost: %v", err)
	}
}

func TestSearchCoversEverySearchableField(t *testing.T) {
	f := newFixture(t)
	found := events.MessageCreated{
		MessageId:         "msg_found",
		ConversationId:    "cnv_1",
		AuthorDisplayName: "Quinn Zeller",
		AuthorAddress:     "Rowan@Yonder.Example",
		Body:              "the quarterly forecast",
		Subject:           "Harvest plans",
		FileNames:         []string{"ledger-final.pdf"},
		AttachmentIds:     []string{"att_1"},
		CreatedAt:         "2026-11-05T10:00:00Z",
	}
	f.create(domainA, found)
	f.create(domainA, message("msg_other", "cnv_1", "nothing to see", "2026-11-04T10:00:00Z"))
	cases := map[string]string{
		"body":             "forecast",
		"subject":          "harvest",
		"sender name":      "zeller",
		"sender address":   "rowan@yonder.example",
		"file name":        "ledger",
		"file extension":   "pdf",
		"prefix":           "quar",
		"two words":        "Quinn FORE",
		"case insensitive": "HARVEST",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			sameIds(t, f.searchIds(tokenA, "q", text), "msg_found")
		})
	}
	sameIds(t, f.searchIds(tokenA, "q", "forecast missingword"))
}

func TestSearchWithEmptyTextMatchesEveryReadableMessage(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "one", "2026-11-05T10:00:00Z"))
	f.create(domainA, message("msg_2", "cnv_2", "two", "2026-11-06T10:00:00Z"))
	f.create(domainA, message("msg_3", "cnv_secret", "three", "2026-11-07T10:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", ""), "msg_2", "msg_1")
}

func TestSearchWithNothingIndexedFindsNothing(t *testing.T) {
	f := newFixture(t)
	page := f.search(tokenA)
	if len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("got %+v", page)
	}
}

func pagedFixture(t *testing.T) *fixture {
	f := newFixture(t)
	f.create(domainA, message("msg_oct", "cnv_1", "budget", "2026-10-30T10:00:00Z"))
	f.create(domainA, message("msg_nov2", "cnv_1", "budget", "2026-11-02T23:30:00Z"))
	f.create(domainA, message("msg_nov5", "cnv_2", "budget", "2026-11-05T10:00:00Z"))
	return f
}

func TestSearchListsNewestFirstAndPagesAcrossMonths(t *testing.T) {
	f := pagedFixture(t)
	first := f.search(tokenA, "q", "budget", "limit", "2")
	sameIds(t, messageIds(first), "msg_nov5", "msg_nov2")
	if first.NextCursor == "" {
		t.Fatal("the first page has no cursor")
	}
	second := f.search(tokenA, "q", "budget", "limit", "2", "cursor", first.NextCursor)
	sameIds(t, messageIds(second), "msg_oct")
	if second.NextCursor != "" {
		t.Fatalf("the last page has a cursor %q", second.NextCursor)
	}
}

func TestSearchPagesOneHitAtATime(t *testing.T) {
	f := pagedFixture(t)
	seen := []string{}
	cursor := ""
	for range 3 {
		page := f.search(tokenA, "limit", "1", "cursor", cursor)
		seen = append(seen, messageIds(page)...)
		cursor = page.NextCursor
	}
	sameIds(t, seen, "msg_nov5", "msg_nov2", "msg_oct")
	if cursor != "" {
		t.Fatalf("cursor after the last hit: %q", cursor)
	}
}

func TestSearchHitCarriesConversationTimeAndSnippet(t *testing.T) {
	f := pagedFixture(t)
	page := f.search(tokenA, "q", "budget", "limit", "1")
	hit := page.Items[0]
	if hit.ConversationId != "cnv_2" || hit.CreatedAt != "2026-11-05T10:00:00.000000Z" || hit.Snippet != "budget" {
		t.Fatalf("got %+v", hit)
	}
}

func TestSearchFilters(t *testing.T) {
	f := newFixture(t)
	withFile := message("msg_file", "cnv_2", "budget", "2026-11-05T10:00:00Z")
	withFile.AttachmentIds = []string{"att_1"}
	withFile.FileNames = []string{"plan.pdf"}
	withFile.AuthorMemberId = "mem_bea"
	withFile.AuthorAddress = "bea@a.example"
	email := message("msg_email", "cnv_1", "budget", "2026-11-02T23:30:00Z")
	email.IsEmail = true
	f.create(domainA, withFile)
	f.create(domainA, email)
	f.create(domainA, message("msg_oct", "cnv_1", "budget", "2026-10-30T10:00:00Z"))
	cases := []struct {
		name  string
		pairs []string
		want  []string
	}{
		{"conversation", []string{"conversationId", " cnv_1 "}, []string{"msg_email", "msg_oct"}},
		{"author address any case", []string{"author", "BEA@A.EXAMPLE"}, []string{"msg_file"}},
		{"author member id", []string{"author", "mem_ana"}, []string{"msg_email", "msg_oct"}},
		{"unknown author", []string{"author", "nobody"}, []string{}},
		{"from a day", []string{"from", "2026-11-02"}, []string{"msg_file", "msg_email"}},
		{"from a time", []string{"from", "2026-11-03T00:00:00Z"}, []string{"msg_file"}},
		{"to a day includes the whole day", []string{"to", "2026-11-02"}, []string{"msg_email", "msg_oct"}},
		{"to a time", []string{"to", "2026-11-02T23:00:00Z"}, []string{"msg_oct"}},
		{"from and to", []string{"from", "2026-11-01", "to", "2026-11-03"}, []string{"msg_email"}},
		{"with attachment", []string{"hasAttachment", "true"}, []string{"msg_file"}},
		{"without attachment", []string{"hasAttachment", "false"}, []string{"msg_email", "msg_oct"}},
		{"email", []string{"isEmail", "true"}, []string{"msg_email"}},
		{"not email", []string{"isEmail", "false"}, []string{"msg_file", "msg_oct"}},
		{"file name with attachment", []string{"q", "plan", "hasAttachment", "true"}, []string{"msg_file"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			sameIds(t, f.searchIds(tokenA, test.pairs...), test.want...)
		})
	}
}

func TestSearchRefusesBadFilters(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name  string
		pairs []string
		field string
	}{
		{"limit zero", []string{"limit", "0"}, "limit"},
		{"limit over 100", []string{"limit", "101"}, "limit"},
		{"limit text", []string{"limit", "ten"}, "limit"},
		{"from", []string{"from", "yesterday"}, "from"},
		{"to", []string{"to", "2026-13-45"}, "to"},
		{"hasAttachment", []string{"hasAttachment", "maybe"}, "hasAttachment"},
		{"isEmail", []string{"isEmail", "nope"}, "isEmail"},
		{"cursor", []string{"cursor", "!!!"}, "cursor"},
		{"q too long", []string{"q", strings.Repeat("a", 501)}, "q"},
		{"q too many words", []string{"q", distinctWords(17)}, "q"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.service.Search(tokenA, query(test.pairs...))
			refusal := InvalidRequest{}
			if !errors.As(err, &refusal) || refusal.Field != test.field || refusal.Message == "" {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestSearchAcceptsTheLongestAllowedText(t *testing.T) {
	f := newFixture(t)
	f.search(tokenA, "q", strings.Repeat("a", 500))
	f.search(tokenA, "q", distinctWords(16), "limit", "100")
}

func TestSearchRefusesABadToken(t *testing.T) {
	f := newFixture(t)
	_, err := f.service.Search("Bearer nobody", query())
	if !errors.As(err, &Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSearchRefusesACallerWhoIsNotAMember(t *testing.T) {
	f := newFixture(t)
	f.identifier.callers["Bearer svc"] = oidc.Caller{DomainId: domainA, Roles: []string{"Service"}}
	_, err := f.service.Search("Bearer svc", query())
	refusal := Forbidden{}
	if !errors.As(err, &refusal) || refusal.Message == "" {
		t.Fatalf("got %v", err)
	}
}

func TestSearchIntoWritesThePageAsJson(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "budget", "2026-11-05T10:00:00Z"))
	target := map[string]any{}
	err := f.service.SearchInto(tokenA, query("q", "budget"), &target)
	items, _ := target["items"].([]any)
	if err != nil || len(items) != 1 || target["nextCursor"] != "" {
		t.Fatalf("got %v %v", target, err)
	}
	first, _ := items[0].(map[string]any)
	if first["messageId"] != "msg_1" || first["conversationId"] != "cnv_1" {
		t.Fatalf("got %v", first)
	}
}

func TestSearchIntoPassesARefusalOn(t *testing.T) {
	f := newFixture(t)
	target := Page{}
	err := f.service.SearchInto("", query(), &target)
	if !errors.As(err, &Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSearchIsUnavailableWhenMonthsCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.documents.failNext("CollectionExists")
	_, err := f.service.Search(tokenA, query())
	if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
		t.Fatalf("got %v", err)
	}
}

func TestSearchIsUnavailableWhenAMonthCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "budget", "2026-11-05T10:00:00Z"))
	f.documents.failAt("Command", 1)
	_, err := f.service.Search(tokenA, query())
	if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
		t.Fatalf("got %v", err)
	}
}

func distinctWords(count int) string {
	words := []string{}
	for index := range count {
		words = append(words, "word"+strconv.Itoa(index))
	}
	return strings.Join(words, " ")
}
