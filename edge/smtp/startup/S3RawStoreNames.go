package startup

type S3RawStoreNames struct{}

func (S3RawStoreNames) Endpoint() string {
	return "MESAIL_RAW_STORE_S3_ENDPOINT"
}

func (S3RawStoreNames) Bucket() string {
	return "MESAIL_RAW_STORE_S3_BUCKET"
}

func (S3RawStoreNames) Region() string {
	return "MESAIL_RAW_STORE_S3_REGION"
}

func (S3RawStoreNames) AccessKeyId() string {
	return "MESAIL_RAW_STORE_S3_ACCESS_KEY_ID"
}

func (S3RawStoreNames) SecretFile() string {
	return "MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE"
}

func (S3RawStoreNames) TimeoutSeconds() string {
	return "MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS"
}
