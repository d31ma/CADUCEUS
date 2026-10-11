import (
	"strings"
	"time"
)

// TeamUtilsTest covers the Team roster and the shared conversation page.
// @Test(ShellTestSetup, {"partition":"tests"})
type TeamUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *TeamUtilsTest) TestMembersGetInitialsAndStatus() {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	members := TeamUtils{}.Members([]Teammate{
		{Email: "ada.lovelace@x.com", Online: true},
		{Email: "bob@x.com", Name: "Bob", LastSeenAt: "2026-10-10T11:15:00Z"},
		{Email: "cy@x.com", LastSeenAt: "2026-10-08T12:00:00Z"},
		{Email: "dee@x.com"},
	}, now)
	Assert.Equals("AL", members[0].Initials)
	Assert.Equals("Online", members[0].Status)
	Assert.Equals("BO", members[1].Initials)
	Assert.Equals("45 min ago", members[1].Status)
	Assert.Equals("2d ago", members[2].Status)
	Assert.Equals("Offline", members[3].Status)
}

func (test *TeamUtilsTest) TestSharedThreadIsDecorated() {
	thread := ShareUtils{}.Decorated(SharedThread{ExpiresAt: "2026-10-11T08:00:00Z", Messages: []SharedMessage{{From: "partner@x.net", To: []string{"a@x.com", "b@x.com"}, HTMLBody: `<img src="https://tracker.invalid/p.png"><p>Hi</p>`, Attachments: []SharedAttachment{{ID: "att", Filename: "a.txt", Size: 2048}}}}}, time.Now())
	Assert.Equals("1 message", thread.MessageCount)
	Assert.Equals("P", thread.Messages[0].Initial)
	Assert.Equals("a@x.com, b@x.com", thread.Messages[0].ToLabel)
	Assert.Equals("2.0 KB", thread.Messages[0].Attachments[0].SizeLabel)
	Assert.Equals("a.txt", ShareUtils{}.Filename(thread, "att"))
	Assert.Equals("attachment", ShareUtils{}.Filename(thread, "none"))
	Assert.Equals("", ShareUtils{}.Token("/team", "x"))
	Assert.Equals("This shared conversation is unavailable.", ShareUtils{}.Unavailable(false, ""))
}

func (test *TeamUtilsTest) TestRemoteResourcesAreRenamed() {
	document := ProtectedMailUtils{}.Document(`<img src="https://tracker.invalid/p.png"><div style="background:url('http://x.invalid/a.png')">`)
	Assert.IsTrue(len(document) > 0)
	Assert.Equals(false, strings.Contains(document, ` src="https://`))
	Assert.IsTrue(strings.Contains(document, "data-caduceus-blocked-src"))
	Assert.IsTrue(strings.Contains(document, "url('about:blank#"))
	Assert.Equals("", ProtectedMailUtils{}.Document(""))
}
