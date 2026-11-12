import (
	"errors"
	"os"
	"strings"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotifySettingsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotifySettingsServiceTest) TestOutboxReadsTheProducerAndSubscriber() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	settings, err := (&NotifySettingsService{}).Outbox()
	Assert.IsTrue(err == nil, "outbox settings missing")
	Assert.Equals("notify", settings.Producer)
	Assert.Equals("mail-out", settings.Subscribers[0].Name)
}

func (test *NotifySettingsServiceTest) TestOutboxRefusesMissingSettings() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := (&NotifySettingsService{}).Outbox()
	unavailable := &StorageUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable))
}

func (test *NotifySettingsServiceTest) TestOptionsReadsThePushHosts() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	options := (&NotifySettingsService{}).Options()
	Assert.Equals(4, len(options.Push.Hosts))
	Assert.Equals("notifications", options.Summary.Sender)
}

func (test *NotifySettingsServiceTest) TestSnapshotIsTheWholeDocument() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	Assert.IsTrue(strings.HasPrefix((&NotifySettingsService{}).snapshot(), `{"yon":`))
}
