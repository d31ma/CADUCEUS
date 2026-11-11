import "os"

// @Test(DirectoryTestSetup, {"partition":"tests"})
type SettingsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SettingsServiceTest) TestSettingsAreReadFromTheYonDirectoryNamespace() {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"directory":{"keycloakurl":"http://keycloak:8080/idp","dkimselector":"mesail"}}}`)
	settings := (&SettingsService{}).Settings()
	Assert.Equals("http://keycloak:8080/idp", settings.KeycloakUrl)
	Assert.Equals("mesail", settings.DkimSelector)
}

func (test *SettingsServiceTest) TestBrokenJsonGivesEmptySettings() {
	os.Setenv("YON_APPLICATION_JSON", "{")
	Assert.Equals("", (&SettingsService{}).Settings().KeycloakUrl)
}
