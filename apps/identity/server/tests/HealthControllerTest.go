import "os"

// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) withOrigins(origins string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"identity":{"passkeyorigins":"`+origins+`"}}}`)
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	test.withOrigins("https://localhost")
	Assert.Equals("identity", HealthTestSetup{}.Health().GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	test.withOrigins("https://mail.example.com android:apk-key-hash:abc")
	Assert.Equals("ok", HealthTestSetup{}.Health().GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedForAnOriginWithAnotherHost() {
	test.withOrigins("https://mail.example.com https://other.test")
	Assert.Equals("degraded", HealthTestSetup{}.Health().GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedForPlainHttpOutsideLocalhost() {
	test.withOrigins("http://mail.example.com")
	Assert.Equals("degraded", HealthTestSetup{}.Health().GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedWithoutOrigins() {
	test.withOrigins("")
	Assert.Equals("degraded", HealthTestSetup{}.Health().GetHealth().Status)
}
