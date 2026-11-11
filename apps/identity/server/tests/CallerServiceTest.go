import "net/http"

// @Test(HealthTestSetup, {"partition":"tests"})
type CallerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallerServiceTest) TestFromRefusesARequestWithoutAToken() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	_, err := (&CallerService{}).From(request)
	Assert.IsTrue(err != nil)
}
