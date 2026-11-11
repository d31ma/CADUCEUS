import (
	"cmp"
	"encoding/json"
	"os"
	"time"
)

// @Service
type SettingsService struct {
}

func (service *SettingsService) Settings() AttachmentSettings {
	application := ApplicationSettings{}
	json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &application)
	return service.withDefaults(application.Yon.Attachments)
}

func (service *SettingsService) LinkExpiry(settings AttachmentSettings) time.Time {
	return time.Now().UTC().Add(time.Duration(settings.Link.Seconds) * time.Second).Truncate(time.Second)
}

func (service *SettingsService) withDefaults(settings AttachmentSettings) AttachmentSettings {
	settings.Adapter = cmp.Or(settings.Adapter, service.localAdapter())
	settings.Bucket = cmp.Or(settings.Bucket, service.defaultBucket())
	settings.Region = cmp.Or(os.Getenv("AWS_REGION"), settings.Region, service.defaultRegion())
	settings.Endpoint = cmp.Or(os.Getenv("AWS_ENDPOINT_URL_S3"), os.Getenv("AWS_ENDPOINT_URL"), settings.Endpoint)
	settings.Link.Seconds = cmp.Or(settings.Link.Seconds, service.defaultLinkSeconds())
	settings.Limit.Bytes = cmp.Or(settings.Limit.Bytes, service.defaultLimitBytes())
	return settings
}

func (service *SettingsService) localAdapter() string {
	return "local"
}

func (service *SettingsService) defaultBucket() string {
	return "mesail-attachments"
}

func (service *SettingsService) defaultRegion() string {
	return "us-east-1"
}

func (service *SettingsService) defaultLinkSeconds() int64 {
	return 300
}

func (service *SettingsService) defaultLimitBytes() int64 {
	return 26214400
}
