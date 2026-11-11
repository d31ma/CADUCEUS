import "mesail.dev/packages/webpush"

// @Test(JobsTestSetup, {"partition":"tests"})
type PushTargetsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushTargetsServiceTest) TestForPlatformsGivesTheWebTarget() {
	setup := JobsTestSetup{}
	setup.Realms(setup.Domain())
	targets, err := JobsTestSetup{}.Targets(nil).ForPlatforms()
	_, found := targets[webpush.Platforms{}.Web()]
	Assert.IsTrue(err == nil, "targets failed")
	Assert.IsTrue(found, "no web target")
}

func (test *PushTargetsServiceTest) TestForPlatformsIsEmptyWithoutVapidKeys() {
	setup := JobsTestSetup{}
	setup.Realms(setup.Domain())
	JobsTestSetup{}.WithoutVapid()
	targets, err := JobsTestSetup{}.Targets(nil).ForPlatforms()
	Assert.IsTrue(err != nil, "missing keys were accepted")
	Assert.Equals(0, len(targets))
}
