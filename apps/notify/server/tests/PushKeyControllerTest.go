import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type PushKeyControllerTest struct {
	behaviors *BehaviorContext
}

func (test *PushKeyControllerTest) TestGetPushKeyGivesAKey() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	key, err := (&PushKeyController{pushKeys: setup.PushKeys()}).GetPushKey(setup.Member(member, "member-1"))
	Assert.IsTrue(err == nil, "key failed")
	Assert.IsTrue(key.PublicKey != "", "no key")
}

func (test *PushKeyControllerTest) TestGetPushKeyRefusesAMissingToken() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	_, err := (&PushKeyController{pushKeys: NotifyTestSetup{}.PushKeys()}).GetPushKey("")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
}

func (test *PushKeyControllerTest) TestGetPushKeyIsUnavailableWhenPushIsTurnedOff() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	token := setup.Member(member, "member-1")
	setup.ChangePush(func(push map[string]any) { push["enabled"] = false })
	_, err := (&PushKeyController{pushKeys: setup.PushKeys()}).GetPushKey(token)
	unavailable := &PushNotConfigured{}
	Assert.IsTrue(errors.As(err, &unavailable))
}
