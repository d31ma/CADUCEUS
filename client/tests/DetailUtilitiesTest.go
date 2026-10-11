import (
	"strings"
	"time"
)

// DetailUtilitiesTest covers the reader's pure utilities.
// @Test(ShellTestSetup, {"partition":"tests"})
type DetailUtilitiesTest struct {
	behaviors *BehaviorContext
}

func (test *DetailUtilitiesTest) at(text string) time.Time {
	moment, _ := time.Parse("2006-01-02T15:04", text)
	return moment
}

func (test *DetailUtilitiesTest) TestSnoozePresetsFollowMailspring() {
	thursday := test.at("2026-10-08T21:30")
	Assert.Equals("2026-10-09T00:00", SnoozePresetsUtils{}.Preset("later-today", thursday).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-09T00:00", SnoozePresetsUtils{}.Preset("tonight", thursday).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-08T20:00", SnoozePresetsUtils{}.Preset("tonight", test.at("2026-10-08T10:00")).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-09T09:00", SnoozePresetsUtils{}.Preset("tomorrow", thursday).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-10T09:00", SnoozePresetsUtils{}.Preset("this-weekend", thursday).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-17T09:00", SnoozePresetsUtils{}.Preset("this-weekend", test.at("2026-10-10T10:00")).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-12T09:00", SnoozePresetsUtils{}.Preset("next-week", thursday).Format("2006-01-02T15:04"))
	Assert.Equals("2026-10-12T09:00", SnoozePresetsUtils{}.Preset("next-week", test.at("2026-10-11T10:00")).Format("2006-01-02T15:04"))
	Assert.Equals("2026-11-01T09:00", SnoozePresetsUtils{}.Preset("next-month", thursday).Format("2006-01-02T15:04"))
	Assert.IsTrue(SnoozePresetsUtils{}.Preset("someday", thursday).IsZero())
	Assert.Equals("2026-10-08T21:30:00.000Z", SnoozePresetsUtils{}.Timestamp(thursday))
	Assert.Equals(thursday, SnoozePresetsUtils{}.Custom("2026-10-08T21:30", thursday))
}

func (test *DetailUtilitiesTest) TestInvitationTimes() {
	Assert.Equals("Mon, Oct 12 · All day", InviteUtils{}.When(CalendarInvite{Start: "2026-10-12T00:00:00Z", End: "2026-10-13T00:00:00Z", AllDay: true}))
	Assert.Equals("Mon, Oct 12 – Tue, Oct 13 · All day", InviteUtils{}.When(CalendarInvite{Start: "2026-10-12T00:00:00Z", End: "2026-10-14T00:00:00Z", AllDay: true}))
	Assert.Equals("", InviteUtils{}.When(CalendarInvite{}))
	Assert.IsTrue(InviteUtils{}.When(CalendarInvite{Start: "2026-10-12T14:00:00Z", End: "2026-10-12T15:00:00Z"}) != "")
}

func (test *DetailUtilitiesTest) TestConditionsAndText() {
	Assert.IsTrue(LogicUtils{}.All(true, true))
	Assert.Equals(false, LogicUtils{}.All(true, false))
	Assert.IsTrue(LogicUtils{}.Any(false, true))
	Assert.Equals(false, LogicUtils{}.Any())
	Assert.Equals("1 message", TextUtils{}.Count(1, "message", "messages"))
	Assert.Equals("3 messages", TextUtils{}.Count(3, "message", "messages"))
	Assert.Equals([]string{"a", "b"}, TextUtils{}.NonEmpty([]string{"a", "", "b"}))
	Assert.Equals("a", TextUtils{}.First([]string{"a", "b"}))
	Assert.Equals("", TextUtils{}.First([]string{}))
}

func (test *DetailUtilitiesTest) TestRoutesAndRecords() {
	Assert.Equals("a/b", RouteUtils{}.EmailId("/email/a%2Fb", ""))
	Assert.Equals("m1", RouteUtils{}.EmailId("/inbox", "?email=m1"))
	Assert.IsTrue(RouteUtils{}.IsStandalone("/email/m1"))
	email := EmailRecordUtils{}.Email(map[string]any{"id": "m1", "folder": "spam", "spamScore": 7.25, "securityWarnings": []any{"reply_to_mismatch"}})
	Assert.Equals("m1", email.ID)
	Assert.IsTrue(email.Spam)
	Assert.Equals("7.2", email.SpamScoreLabel)
	Assert.IsTrue(email.ReplyToMismatch)
	Assert.Equals(0, len(EmailRecordUtils{}.InviteAttachments(map[string]any{})))
	Assert.Equals("a.ics", EmailRecordUtils{}.InviteAttachments(map[string]any{"calendarInvite": map[string]any{"attachment": map[string]any{"id": "i1", "filename": "a.ics"}}})[0].Filename)
}

func (test *DetailUtilitiesTest) TestContentProtectionBlocksRemoteImages() {
	document := ContentProtectionUtils{}.BuildDocument(`<p>Hi<script>alert(1)</script><img src="https://images.example/a.png"></p>`, ProtectionOptions{InlineSources: map[string]string{}, RemoteSources: map[string]string{}, ColorMode: "light"})
	Assert.Equals(1, document.BlockedCount)
	Assert.Equals([]string{"https://images.example/a.png"}, document.RemoteUrls)
	Assert.Equals(false, strings.Contains(document.DocumentHtml, "<script"))
}
