import (
	"errors"
	"time"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotificationsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationsServiceTest) TestListPagesNewestFirst() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	older := setup.Notification(domainId, "member-1", "msg_1")
	older.CreatedAt = time.Now().Add(-time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	setup.NotificationStore().InsertIfAbsent(older)
	setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_2"))
	first, err := setup.Notifications().List(setup.Member(member, "member-1"), "", "1")
	second, again := setup.Notifications().List(setup.Member(member, "member-1"), first.NextCursor, "1")
	Assert.IsTrue(err == nil && again == nil, "list failed")
	Assert.Equals("msg_2", first.Items[0].MessageId)
	Assert.Equals("msg_1", second.Items[0].MessageId)
	Assert.Equals("", second.NextCursor)
	Assert.Equals("ntf_", first.Items[0].NotificationId[:4])
}

func (test *NotificationsServiceTest) TestListRefusesABadLimit() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := setup.Notifications().List(setup.Member(member, "member-1"), "", "500")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals("limit", invalid.Field)
}

func (test *NotificationsServiceTest) TestListRefusesABadCursor() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := setup.Notifications().List(setup.Member(member, "member-1"), "!!", "")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals("cursor", invalid.Field)
}

func (test *NotificationsServiceTest) TestListRefusesAMissingToken() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	_, err := NotifyTestSetup{}.Notifications().List("", "", "")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
}

func (test *NotificationsServiceTest) TestMarkReadMarksTheCallersNotification() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_3"))
	err := setup.Notifications().MarkRead(setup.Member(member, "member-1"), "ntf_"+stored.Id)
	found, _, _ := setup.NotificationStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "mark failed")
	Assert.IsTrue(found.ReadAt != "", "not read")
}

func (test *NotificationsServiceTest) TestMarkReadHidesAnotherMembersNotification() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_4"))
	err := setup.Notifications().MarkRead(setup.Member(member, "member-2"), "ntf_"+stored.Id)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *NotificationsServiceTest) TestMarkReadHidesAnotherDomainsNotification() {
	setup := NotifyTestSetup{}
	other := setup.Domain()
	member, _ := setup.Realms(setup.Domain())
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(other, "member-1", "msg_5"))
	err := setup.Notifications().MarkRead(setup.Member(member, "member-1"), "ntf_"+stored.Id)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *NotificationsServiceTest) TestMarkReadRefusesAWrongPrefix() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := setup.Notifications().MarkRead(setup.Member(member, "member-1"), "msg_1")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}
