import (
	"os"
	"time"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type SettingsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SettingsServiceTest) TestSettingsFallBackToDefaults() {
	os.Setenv("YON_APPLICATION_JSON", "")
	settings := (&SettingsService{}).Settings()
	Assert.Equals("local", settings.Adapter)
	Assert.Equals("mesail-attachments", settings.Bucket)
	Assert.Equals(int64(300), settings.Link.Seconds)
	Assert.Equals(int64(26214400), settings.Limit.Bytes)
}

func (test *SettingsServiceTest) TestSettingsReadTheAttachmentsBlock() {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"attachments":{"adapter":"s3","bucket":"files","link":{"seconds":60},"limit":{"bytes":5}}}}`)
	settings := (&SettingsService{}).Settings()
	Assert.Equals("s3", settings.Adapter)
	Assert.Equals("files", settings.Bucket)
	Assert.Equals(int64(60), settings.Link.Seconds)
	Assert.Equals(int64(5), settings.Limit.Bytes)
}

func (test *SettingsServiceTest) TestLinkExpiryIsTheLinkLifetimeFromNow() {
	expiry := (&SettingsService{}).LinkExpiry(AttachmentSettings{Link: LinkSettings{Seconds: 120}})
	Assert.IsTrue(expiry.After(time.Now().Add(100*time.Second)), "expiry too early")
	Assert.IsTrue(expiry.Before(time.Now().Add(121*time.Second)), "expiry too late")
}
