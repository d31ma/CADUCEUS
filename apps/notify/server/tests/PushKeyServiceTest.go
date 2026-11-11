import (
	"errors"
	"os"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type PushKeyServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushKeyServiceTest) TestKeyGivesThePublicKey() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	key, err := setup.PushKeys().Key(setup.Member(member, "member-1"))
	Assert.IsTrue(err == nil, "key failed")
	Assert.Equals(87, len(key.PublicKey))
}

func (test *PushKeyServiceTest) TestKeyIsUnavailableWithoutVapidKeys() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	token := setup.Member(member, "member-1")
	document, _ := member.ApplicationJson()
	os.Setenv("YON_APPLICATION_JSON", document)
	key, err := setup.PushKeys().Key(token)
	unavailable := &PushNotConfigured{}
	Assert.IsTrue(errors.As(err, &unavailable))
	Assert.Equals("", key.PublicKey)
}
