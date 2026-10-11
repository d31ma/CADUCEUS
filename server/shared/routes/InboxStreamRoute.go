import (
	"net/http"
	"sort"
	"strconv"
	"time"
)

// InboxStreamRoute answers the /inbox/stream socket: live new mail for the
// signed-in mailbox, as the JavaScript Server-Sent Events stream sent it.
//
// The socket is bounded and cursor-based. A browser cannot set headers on a
// WebSocket, so the bearer token comes as ?token= (an Authorization header
// still wins), and the client passes its cursor as ?since=. On open the
// stream announces the cursor, replays the oldest bounded slice newer than
// each source position, then polls, sending each message as new-email with
// the cursor that follows it, or a ping when there is none. After
// STREAM_MAX_MS it sends reconnect and closes; the client reconnects with
// its latest cursor. A refused token gets one unauthorized message and the
// socket closes.
type InboxStreamRoute struct {
	Access     *AccessHelper
	Categories *CategoryMappingService
	Emails     *EmailsHelper
}

func (InboxStreamRoute) defaultWindow() string { return "25000" }

func (InboxStreamRoute) defaultPoll() string { return "2000" }

func (InboxStreamRoute) defaultReplay() string { return "50" }

func (InboxStreamRoute) providerPage() int { return 1000 }

func (InboxStreamRoute) lastProviderOffset() int { return 100_000 }

// Open runs one stream until its window ends or the caller is refused.
func (route InboxStreamRoute) Open(session *SocketSession) error {
	request := session.Request()
	caller, token, refusal := route.caller(request)
	if refusal != nil {
		if err := session.Send(route.message("unauthorized", map[string]any{"error": "Authentication required"}, "")); err != nil {
			return err
		}
		return session.Close()
	}
	since := request.URL.Query().Get("since")
	if since == "" {
		since = EpochTime{}.NowText()
	}
	cursor := InboxStreamCursor{}.Parsed(since)
	if err := session.Send(route.message("connected", map[string]any{"since": cursor.Text()}, "")); err != nil {
		return err
	}
	if err := route.poll(session, caller, token, &cursor); err != nil {
		if err := session.Send(route.message("error", map[string]any{"message": err.Error()}, "")); err != nil {
			return err
		}
	}
	if err := session.Send(route.message("reconnect", map[string]any{"since": cursor.Text()}, "")); err != nil {
		return err
	}
	return session.Close()
}

// Received ignores what the browser sends: the stream only speaks.
func (InboxStreamRoute) Received(*SocketSession, InboxStreamMessage) error {
	return nil
}

// caller authenticates the request as the HTTP routes do, with ?token= when
// it carries no bearer header.
func (route InboxStreamRoute) caller(request *http.Request) (RouteCaller, string, *http.Response) {
	token := RouteAccess{}.Bearer(request)
	signed := request
	if token == "" {
		token = request.URL.Query().Get("token")
		signed = request.Clone(request.Context())
		signed.Header.Set("Authorization", "Bearer "+token)
	}
	caller, refusal := route.Access.Signed(signed)
	return caller, token, refusal
}

func (route InboxStreamRoute) poll(session *SocketSession, caller RouteCaller, token string, cursor *InboxStreamCursor) error {
	deadline := time.Now().Add(time.Duration(route.setting("STREAM_MAX_MS", route.defaultWindow())) * time.Millisecond)
	limit := route.setting("STREAM_REPLAY_LIMIT", route.defaultReplay())
	for time.Now().Before(deadline) {
		hosted, err := route.hosted(caller, cursor.Hosted, limit)
		if err != nil {
			return err
		}
		// A provider failure keeps the provider position, so a later
		// reconnect replays its gap even when hosted mail moves on.
		external, _ := route.external(token, cursor.External, limit)
		fresh := append(hosted, external...)
		order := TextOrder{}
		sort.SliceStable(fresh, func(left, right int) bool {
			if byTime := order.Compare(route.text(fresh[left]["receivedAt"]), route.text(fresh[right]["receivedAt"])); byTime != 0 {
				return byTime < 0
			}
			return order.Compare(route.text(fresh[left]["id"]), route.text(fresh[right]["id"])) < 0
		})
		fresh = fresh[:min(len(fresh), limit)]
		for _, email := range fresh {
			position := &cursor.Hosted
			if email["external"] == true {
				position = &cursor.External
			}
			if at := route.text(email["receivedAt"]); at != "" {
				position.At = at
			}
			position.Id = route.text(email["id"])
			if err := session.Send(route.message("new-email", route.summary(email), cursor.Text())); err != nil {
				return err
			}
		}
		if len(fresh) == 0 {
			if err := session.Send(route.message("ping", map[string]any{}, "")); err != nil {
				return err
			}
		}
		time.Sleep(time.Duration(route.setting("STREAM_POLL_MS", route.defaultPoll())) * time.Millisecond)
	}
	return nil
}

// hosted answers the caller's inbox mail after a position, oldest first.
func (route InboxStreamRoute) hosted(caller RouteCaller, position InboxStreamPosition, limit int) ([]map[string]any, error) {
	domains := []string{caller.Domain}
	mappings, err := route.Categories.ForOwner(caller.Email)
	if err != nil {
		return nil, err
	}
	emails, err := route.Emails.List(domains, EmailListFilters{
		Folder: "inbox", FolderByDomain: mappings.FolderFilter(domains, "inbox"),
		Since: InboxStreamCursor{}.ReplaySince(position.At), OwnerEmail: caller.Email,
	})
	if err != nil {
		return nil, err
	}
	fresh := []map[string]any{}
	presented := mappings.PresentEmails(emails)
	for index := len(presented) - 1; index >= 0; index-- {
		if (InboxStreamCursor{}).After(presented[index], position) {
			fresh = append(fresh, presented[index])
		}
	}
	return fresh[:min(len(fresh), limit)], nil
}

// external answers the caller's connected-account inbox mail after a
// position, oldest first.
func (route InboxStreamRoute) external(token string, position InboxStreamPosition, limit int) ([]map[string]any, error) {
	unified := UnifiedMail{Accounts: ExternalAccounts{}.FromSettings()}
	fresh := []map[string]any{}
	for offset := 0; len(fresh) < limit && offset <= route.lastProviderOffset(); {
		from := position.At
		page, err := unified.Summaries(token, SummaryFilter{FolderRole: "inbox", ReceivedAtFrom: &from, OldestFirst: true, Limit: route.providerPage(), Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, email := range page {
			if (InboxStreamCursor{}).After(email, position) {
				fresh = append(fresh, email)
			}
		}
		if len(page) < route.providerPage() {
			break
		}
		offset += len(page)
	}
	return fresh[:min(len(fresh), limit)], nil
}

// summary answers the light row a new-email message carries: never bodies
// or attachments, and no provider credentials.
func (InboxStreamRoute) summary(email map[string]any) map[string]any {
	summary := map[string]any{}
	fields := []string{"id", "domain", "sender", "recipient", "subject", "folder", "systemFolder", "read", "starred", "receivedAt"}
	if email["external"] == true {
		summary["external"] = true
		fields = append(fields, "providerAccountId", "providerStableId", "mailboxAddress", "mailboxLabel", "mailboxColor")
	}
	for _, field := range fields {
		if value, present := email[field]; present {
			summary[field] = value
		}
	}
	if score, present := email["spamScore"]; present && score != nil {
		summary["spamScore"] = score
	}
	return summary
}

func (InboxStreamRoute) message(event string, data map[string]any, cursor string) InboxStreamMessage {
	message := InboxStreamMessage{Event: event, Data: data}
	if cursor != "" {
		message.Cursor = &cursor
	}
	return message
}

// setting answers a whole-number stream setting; text that is no number is
// 0, as Number() made it NaN.
func (InboxStreamRoute) setting(name string, fallback string) int {
	value := Settings{}.Value(name)
	if value == "" {
		value = fallback
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number != number || number < 0 {
		return 0
	}
	return int(number)
}

func (InboxStreamRoute) text(value any) string {
	return TextValue{}.Of(TextValue{}.Or(value, ""))
}
