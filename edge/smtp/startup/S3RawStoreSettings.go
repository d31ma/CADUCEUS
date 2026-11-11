package startup

import "time"

type S3RawStoreSettings struct {
	Endpoint    string
	Bucket      string
	Region      string
	AccessKeyId string
	SecretFile  string
	Timeout     time.Duration
}
