import (
	"errors"
	"net/http/httptest"
	"net/url"
	"time"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotificationsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationsServiceTest) list(authorization string, cursor string, limit string) (NotificationPage, error) {
	request := httptest.NewRequest("GET", "/api/v1/notify/notifications?"+url.Values{"cursor": {cursor}, "limit": {limit}}.Encode(), nil)
	request.Header.Set("Authorization", authorization)
	return NotifyTestSetup{}.Notifications().Page(request)
}

func (test *NotificationsServiceTest) TestListPagesNewestFirst() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	older := setup.Notification(domainId, "member-1", "msg_1")
	older.CreatedAt = time.Now().Add(-time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	setup.NotificationStore().InsertIfAbsent(older)
	setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_2"))
	first, err := test.list(setup.Member(member, "member-1"), "", "1")
	second, again := test.list(setup.Member(member, "member-1"), first.NextCursor, "1")
	Assert.IsTrue(err == nil && again == nil, "list failed")
	Assert.Equals("msg_2", first.Items[0].MessageId)
	Assert.Equals("msg_1", second.Items[0].MessageId)
	Assert.Equals("", second.NextCursor)
	Assert.Equals("ntf_", first.Items[0].NotificationId[:4])
}

func (test *NotificationsServiceTest) TestListRefusesABadLimit() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := test.list(setup.Member(member, "member-1"), "", "500")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals(4508, invalid.Number())
	Assert.Equals("limit", invalid.Field())
}

func (test *NotificationsServiceTest) TestListRefusesABadCursor() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := test.list(setup.Member(member, "member-1"), "!!", "")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals(4509, invalid.Number())
	Assert.Equals("cursor", invalid.Field())
}

func (test *NotificationsServiceTest) TestListRefusesAMissingToken() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	_, err := test.list("", "", "")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
	Assert.Equals(4501, refused.Number())
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
	Assert.Equals(4510, missing.Number())
}

func (test *NotificationsServiceTest) TestMarkReadHidesAnotherDomainsNotification() {
	setup := NotifyTestSetup{}
	other := setup.Domain()
	member, _ := setup.Realms(setup.Domain())
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(other, "member-1", "msg_5"))
	err := setup.Notifications().MarkRead(setup.Member(member, "member-1"), "ntf_"+stored.Id)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4510, missing.Number())
}

func (test *NotificationsServiceTest) TestMarkReadRefusesAWrongPrefix() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := setup.Notifications().MarkRead(setup.Member(member, "member-1"), "msg_1")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4510, missing.Number())
}
