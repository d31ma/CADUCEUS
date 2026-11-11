import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type OutboxControllerTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxControllerTest) controller() *OutboxController {
	return &OutboxController{relay: NotifyTestSetup{}.Relay(outbox.NewRecordingPusher())}
}

func (test *OutboxControllerTest) TestPostRelayRefusesADomainService() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := test.controller().PostRelay(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxControllerTest) TestGetDeadRefusesADomainService() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := test.controller().GetDead(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxControllerTest) TestPostRetryRefusesADomainService() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := test.controller().PostRetry(setup.Service(member), RetryRequest{})
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}
