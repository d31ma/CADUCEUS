import "errors"

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type CallerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallerServiceTest) TestCallerRefusesAForgedToken() {
	_, err := (&CallerService{}).Caller("Bearer forged")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}
