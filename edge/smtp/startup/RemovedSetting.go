package startup

type RemovedSetting struct {
	Name string
}

func (removed RemovedSetting) Error() string {
	return "the environment variable " + removed.Name + " was removed with the S3 raw store (ADR 0007): remove it and keep raw messages in the folder MESAIL_RAW_STORE_DIR"
}
