package blobs

import "cmp"

type S3Settings struct {
	Endpoint        string
	PublicEndpoint  string
	Region          string
	Bucket          string
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

func (settings S3Settings) BaseUrl() string {
	return settings.Endpoint
}

func (settings S3Settings) PublicUrl() string {
	return cmp.Or(settings.PublicEndpoint, settings.Endpoint)
}

func (settings S3Settings) Check() error {
	if settings.Endpoint == "" {
		return MissingEndpoint{}
	}
	return nil
}
