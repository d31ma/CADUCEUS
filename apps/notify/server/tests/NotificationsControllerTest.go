import (
	"errors"
	"net/http"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotificationsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationsControllerTest) TestGetNotificationsReadsTheQuery() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_1"))
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/notify/notifications?limit=5", nil)
	request.Header.Set("Authorization", setup.Member(member, "member-1"))
	page, err := (&NotificationsController{notifications: setup.Notifications()}).GetNotifications(request)
	Assert.IsTrue(err == nil, "list failed")
	Assert.Equals(1, len(page.Items))
}

func (test *NotificationsControllerTest) TestPutReadAnswersNotFoundForAnUnknownId() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := (&NotificationsController{notifications: setup.Notifications()}).PutRead(setup.Member(member, "member-1"), "ntf_unknown")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}
