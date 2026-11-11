package blobs

type S3Settings struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

func (settings S3Settings) BaseUrl() string {
	return settings.endpointOrAws(settings.Endpoint)
}

func (settings S3Settings) endpointOrAws(endpoint string) string {
	if endpoint == "" {
		return "https://s3." + settings.Region + ".amazonaws.com"
	}
	return endpoint
}
