package search

import (
	"errors"
	"testing"
)

func TestCollectionNamesUseTheDomainKey(t *testing.T) {
	collections := Collections{}
	cases := map[string]string{
		collections.Documents("example.com", "202611"):   "search-index-a379a6f6eeafb9a5-202611",
		collections.Documents(" Example.COM ", "202611"): "search-index-a379a6f6eeafb9a5-202611",
		collections.Months("example.com"):                "search-months-a379a6f6eeafb9a5",
		collections.Locations("example.com"):             "search-locations-a379a6f6eeafb9a5",
		collections.SeenEvents("example.com"):            "events-seen-a379a6f6eeafb9a5",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	if collections.Months(domainA) == collections.Months(domainB) {
		t.Fatal("two domains share a collection")
	}
}

func TestSettingsReaderReadsTheSearchNamespace(t *testing.T) {
	environment := func(name string) string {
		if name != "YON_APPLICATION_JSON" {
			return ""
		}
		return `{"yon":{"search":{"messagingurl":"http://messaging:8080","channelsurl":"http://channels:8080"}}}`
	}
	settings := SettingsReader{Environment: environment}.Read()
	if settings.MessagingUrl != "http://messaging:8080" || settings.ChannelsUrl != "http://channels:8080" {
		t.Fatalf("got %+v", settings)
	}
}

func TestSettingsReaderGivesEmptySettingsForBadJson(t *testing.T) {
	settings := SettingsReader{Environment: func(string) string { return "{not json" }}.Read()
	if settings != (Settings{}) {
		t.Fatalf("got %+v", settings)
	}
}

func TestServiceFactoryBuildsAServiceThatRefusesAMissingToken(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "")
	environment := func(string) string {
		return `{"yon":{"search":{"messagingurl":"http://127.0.0.1:1","channelsurl":"http://127.0.0.1:1"}}}`
	}
	service := ServiceFactory{Environment: environment}.Service(newFakeDocuments())
	_, err := service.Search("", query())
	if !errors.As(err, &Unauthenticated{}) {
		t.Fatalf("got %#v", err)
	}
}

func TestErrorMessages(t *testing.T) {
	cause := errors.New("disk full")
	cases := map[string]error{
		"bad limit":                    InvalidRequest{Message: "bad limit", Field: "limit"},
		"no access":                    Forbidden{Message: "no access"},
		"the access token was refused": Unauthenticated{},
		"the search index is not available: disk full": Unavailable{Message: "the search index is not available", Cause: cause},
		"the channels service answered status 503":     MembershipFailed{Service: "channels", Status: 503},
	}
	for want, err := range cases {
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err.Error(), want)
		}
	}
}

func TestUnavailableUnwrapsItsCause(t *testing.T) {
	cause := errors.New("disk full")
	failure := Unavailable{Message: "down", Cause: cause}
	if failure.Unwrap() != cause || !errors.Is(failure, cause) {
		t.Fatalf("got %v", failure.Unwrap())
	}
	if (Unavailable{Message: "down"}).Unwrap() != nil {
		t.Fatal("an Unavailable without a cause unwraps to something")
	}
}

func TestEventFailuresPassUnknownErrorsOn(t *testing.T) {
	cause := errors.New("other")
	if (EventFailures{}).Translated(cause) != cause || (EventFailures{}).Translated(nil) != nil {
		t.Fatal("an unknown error was changed")
	}
}

func TestJsonCopyRefusesAValueThatIsNotJson(t *testing.T) {
	target := map[string]any{}
	err := Json{}.Copy(map[string]any{"value": make(chan int)}, &target)
	refusal := InvalidRequest{}
	if !errors.As(err, &refusal) || refusal.Field != "body" {
		t.Fatalf("got %#v", err)
	}
}

func TestHitSnippetFallsBackToSubjectThenFileNames(t *testing.T) {
	filter := Filter{Limit: 20, Terms: []string{"plan"}}
	page := Hits{}.Page([]Document{
		{MessageId: "msg_subject", Subject: "plan for today"},
		{MessageId: "msg_files", FileNames: []string{"plan.pdf", "notes.txt"}},
	}, filter)
	if page.Items[0].Snippet != "plan for today" || page.Items[1].Snippet != "plan.pdf notes.txt" || page.NextCursor != "" {
		t.Fatalf("got %+v", page)
	}
}

func TestMatchesWhereAddsOnlyTheGivenFilters(t *testing.T) {
	yes := true
	where := Matches{}.Where(Filter{ConversationId: "cnv_1", IsEmail: &yes})
	if len(where) != 3 || where["deleted"] != false || where["conversationId"] != "cnv_1" || where["isEmail"] != true {
		t.Fatalf("got %v", where)
	}
}
