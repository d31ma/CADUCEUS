import "strings"

// @Test(NotifyTestSetup, {"partition":"tests"})
type CollectionNamesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CollectionNamesServiceTest) TestNotificationsEndsWithTheMonth() {
	name := (&CollectionNamesService{}).Notifications("example.com", "202611")
	Assert.IsTrue(strings.HasPrefix(name, "notifications-"), name)
	Assert.IsTrue(strings.HasSuffix(name, "-202611"), name)
}

func (test *CollectionNamesServiceTest) TestDomainCollectionsUseTheirBase() {
	names := &CollectionNamesService{}
	Assert.IsTrue(strings.HasPrefix(names.PushSubscriptions("example.com"), "push-subscriptions-"))
	Assert.IsTrue(strings.HasPrefix(names.MemberSettings("example.com"), "notify-settings-"))
	Assert.IsTrue(strings.HasPrefix(names.Threads("example.com"), "notify-threads-"))
	Assert.IsTrue(strings.HasPrefix(names.Summaries("example.com"), "notify-summaries-"))
	Assert.IsTrue(strings.HasPrefix(names.Fanout("example.com"), "notify-fanout-"))
	Assert.IsTrue(strings.HasPrefix(names.SeenEvents("example.com"), "events-seen-"))
}

func (test *CollectionNamesServiceTest) TestInstallWideCollectionsHaveFixedNames() {
	names := &CollectionNamesService{}
	Assert.Equals("notify-domains", names.Domains())
	Assert.Equals("outbox-collections", names.OutboxCollections())
}

func (test *CollectionNamesServiceTest) TestDomainsGetDifferentCollections() {
	names := &CollectionNamesService{}
	Assert.IsTrue(names.Summaries("a.example.com") != names.Summaries("b.example.com"))
}
