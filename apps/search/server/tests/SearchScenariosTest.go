import (
	"errors"
	"net/url"

	"mesail.dev/packages/search"
)

// @Test(SearchTestSetup, {"partition":"tests"})
type SearchScenariosTest struct {
	behaviors *BehaviorContext
}

func (test *SearchScenariosTest) TestAMemberNeverSeesAnotherDomainsMessageWithTheSameWords() {
	world := SearchTestSetup{}.World()
	other := "other-" + world.Domain
	ana := world.Member(world.Domain, "ana", "cnv_shared")
	eve := world.Member(other, "eve", "cnv_shared")
	anaMessage := world.Created(world.Domain, world.Message("cnv_shared", "quarterly secret numbers", "2026-11-10T10:00:00Z"))
	eveMessage := world.Created(other, world.Message("cnv_shared", "quarterly secret numbers", "2026-11-10T11:00:00Z"))
	Assert.Equals([]string{anaMessage.MessageId}, world.Ids(world.Search(ana, url.Values{"q": {"quarterly secret"}})))
	Assert.Equals([]string{eveMessage.MessageId}, world.Ids(world.Search(eve, url.Values{"q": {"quarterly secret"}})))
}

func (test *SearchScenariosTest) TestSearchHidesConversationsTheMemberCannotReadNow() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_open")
	world.Created(world.Domain, world.Message("cnv_private", "salary review", "2026-11-10T10:00:00Z"))
	Assert.Equals(0, len(world.Search(ana, url.Values{"q": {"salary"}}).Items))
}

func (test *SearchScenariosTest) TestSearchFailsClosedWhenMembershipIsUnknown() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_one")
	world.Created(world.Domain, world.Message("cnv_one", "any words", "2026-11-10T10:00:00Z"))
	world.failure = errors.New("messaging is down")
	_, err := world.Service.Search(ana, url.Values{"q": {"words"}})
	Assert.IsTrue(errors.As(err, &search.Unavailable{}), "expected unavailable")
}

func (test *SearchScenariosTest) TestSearchIsNewestFirstAndPagedByCursorAcrossMonths() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_one")
	oldest := world.Created(world.Domain, world.Message("cnv_one", "standup notes", "2026-10-01T09:00:00Z"))
	middle := world.Created(world.Domain, world.Message("cnv_one", "standup notes", "2026-11-01T09:00:00Z"))
	newest := world.Created(world.Domain, world.Message("cnv_one", "standup notes", "2026-11-02T09:00:00Z"))
	first := world.Search(ana, url.Values{"q": {"standup"}, "limit": {"2"}})
	second := world.Search(ana, url.Values{"q": {"standup"}, "limit": {"2"}, "cursor": {first.NextCursor}})
	Assert.Equals([]string{newest.MessageId, middle.MessageId}, world.Ids(first))
	Assert.Equals([]string{oldest.MessageId}, world.Ids(second))
	Assert.Equals("", second.NextCursor)
}

func (test *SearchScenariosTest) TestSearchFiltersByConversationAuthorAndDates() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_one", "cnv_two")
	wanted := world.Created(world.Domain, world.Message("cnv_one", "release checklist", "2026-10-15T10:00:00.5Z"))
	world.Created(world.Domain, world.Message("cnv_two", "release checklist", "2026-10-15T11:00:00Z"))
	world.Created(world.Domain, world.Message("cnv_one", "release checklist", "2026-11-01T00:00:00Z"))
	values := url.Values{"q": {"release"}, "conversationId": {"cnv_one"}, "author": {"ANA@" + world.Domain}, "from": {"2026-10-01"}, "to": {"2026-10-31"}}
	Assert.Equals([]string{wanted.MessageId}, world.Ids(world.Search(ana, values)))
}

func (test *SearchScenariosTest) TestSearchCoversSubjectSenderAndFileNames() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_mail")
	message := world.Message("cnv_mail", "see attached", "2026-11-10T10:00:00Z")
	message.Subject = "Invoice November"
	message.FileNames = []string{"budget-2026.xlsx"}
	message.AttachmentIds = []string{"att_1"}
	message.IsEmail = true
	world.Created(world.Domain, message)
	Assert.Equals(1, len(world.Search(ana, url.Values{"q": {"invoice"}, "isEmail": {"true"}, "hasAttachment": {"true"}}).Items))
	Assert.Equals(1, len(world.Search(ana, url.Values{"q": {"lima"}}).Items))
	Assert.Equals(1, len(world.Search(ana, url.Values{"q": {"budget xlsx"}}).Items))
}

func (test *SearchScenariosTest) TestEditedAndDeletedMessagesFollowTheirEvents() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_one")
	edited := world.Created(world.Domain, world.Message("cnv_one", "meeting at noon", "2026-11-10T10:00:00Z"))
	deleted := world.Created(world.Domain, world.Message("cnv_one", "oops wrong chat", "2026-11-10T10:00:00Z"))
	early := world.Message("cnv_one", "late arrival", "2026-11-10T10:00:00Z")
	world.Edit(world.Domain, edited, "meeting at three")
	world.Delete(world.Domain, deleted)
	world.Delete(world.Domain, early)
	world.Created(world.Domain, early)
	Assert.Equals(0, len(world.Search(ana, url.Values{"q": {"noon"}}).Items))
	Assert.Equals([]string{edited.MessageId}, world.Ids(world.Search(ana, url.Values{"q": {"three"}})))
	Assert.Equals(0, len(world.Search(ana, url.Values{"q": {"oops"}}).Items))
	Assert.Equals(0, len(world.Search(ana, url.Values{"q": {"arrival"}}).Items))
}

func (test *SearchScenariosTest) TestARepeatedEventIsAppliedOnce() {
	world := SearchTestSetup{}.World()
	ana := world.Member(world.Domain, "ana", "cnv_one")
	message := world.Message("cnv_one", "twice sent", "2026-11-10T10:00:00Z")
	envelope := world.Envelope(world.Domain, "message.created", message)
	world.Must(world.Receive(envelope))
	world.Must(world.Receive(envelope))
	world.Created(world.Domain, message)
	Assert.Equals([]string{message.MessageId}, world.Ids(world.Search(ana, url.Values{"q": {"twice"}})))
}
