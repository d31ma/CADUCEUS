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
	Assert.Equals("identity", (&HealthController{}).GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	test.withOrigins("https://mail.example.com android:apk-key-hash:abc")
	Assert.Equals("ok", (&HealthController{}).GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedForAnOriginWithAnotherHost() {
	test.withOrigins("https://mail.example.com https://other.test")
	Assert.Equals("degraded", (&HealthController{}).GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedForPlainHttpOutsideLocalhost() {
	test.withOrigins("http://mail.example.com")
	Assert.Equals("degraded", (&HealthController{}).GetHealth().Status)
}

func (test *HealthControllerTest) TestGetHealthIsDegradedWithoutOrigins() {
	test.withOrigins("")
	Assert.Equals("degraded", (&HealthController{}).GetHealth().Status)
}
