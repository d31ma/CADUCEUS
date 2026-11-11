import "errors"

// @TestSetup
type SearchTestSetup struct{}

func (setup SearchTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["identity"] = setup
	configuration.Http["IdentityClient"] = setup
	configuration.Http["search"] = setup
	configuration.Http["SearchClient"] = setup
	return configuration, nil
}

func (setup SearchTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	switch request.Method + " " + request.Path {
	case "POST /api/v1/identity/session":
		return setup.session(), nil
	case "GET /api/v1/search":
		return setup.searchUnlessRefused(request)
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup SearchTestSetup) Queries(behaviors *BehaviorContext) *SearchQueryService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := &SessionTokenService{identity: NewIdentityClientWith(behaviors)}
	tokens.SignIn(SessionSignInRequest{Address: "ana@example.com", Password: "right"})
	return &SearchQueryService{
		client:   NewSearchClientWith(behaviors),
		tokens:   tokens,
		accounts: &SessionAccountService{CurrentDomain: "example.com"},
		rows:     &SearchRowService{highlights: &SearchHighlightService{}, times: &SearchTimeService{}},
	}
}

func (setup SearchTestSetup) session() Session {
	me := Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}}
	return Session{Me: me, AccessToken: "access-1", ExpiresIn: 60, SessionKey: "key-1"}
}

func (setup SearchTestSetup) searchUnlessRefused(request *BehaviorHttpRequest) (BehaviorValue, error) {
	if request.Headers["Authorization"] != "Bearer access-1" {
		return nil, errors.New("unauthenticated")
	}
	return setup.search(request.Query)
}

func (setup SearchTestSetup) search(query map[string]any) (BehaviorValue, error) {
	switch query["q"].(string) + " " + query["cursor"].(string) {
	case "invoice ":
		return setup.firstPage(query), nil
	case "invoice page-2":
		return SearchHitPage{Items: []SearchHit{setup.hit("m-3", "Old invoice from March")}}, nil
	case "filtered ":
		return setup.filtered(query), nil
	case "nothing ":
		return SearchHitPage{Items: []SearchHit{}}, nil
	}
	return nil, errors.New("search is down")
}

func (setup SearchTestSetup) firstPage(query map[string]any) SearchHitPage {
	if query["hasAttachment"] != "" || query["isEmail"] != "" || query["limit"] == "" {
		return SearchHitPage{Items: []SearchHit{}}
	}
	hits := []SearchHit{setup.hit("m-1", "...the March invoice is ready"), setup.hit("m-2", "Invoices sent")}
	return SearchHitPage{Items: hits, NextCursor: "page-2"}
}

func (setup SearchTestSetup) filtered(query map[string]any) SearchHitPage {
	wanted := map[string]any{"q": "filtered", "conversationId": "c-1", "author": "bo", "from": "2026-11-01", "to": "2026-11-10", "hasAttachment": "true", "isEmail": "true", "cursor": "", "limit": "20"}
	if len(query) != len(wanted) {
		return SearchHitPage{Items: []SearchHit{}}
	}
	return setup.matching(query, wanted)
}

func (setup SearchTestSetup) matching(query map[string]any, wanted map[string]any) SearchHitPage {
	for name, value := range wanted {
		if query[name] != value {
			return SearchHitPage{Items: []SearchHit{}}
		}
	}
	return SearchHitPage{Items: []SearchHit{setup.hit("m-9", "filtered note")}}
}

func (setup SearchTestSetup) hit(messageId string, snippet string) SearchHit {
	return SearchHit{MessageId: messageId, ConversationId: "c-1", Snippet: snippet, CreatedAt: "2026-11-10T08:52:00Z"}
}
