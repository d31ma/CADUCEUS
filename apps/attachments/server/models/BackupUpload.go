type BackupUpload struct {
	Version   int64  `json:"version"`
	UploadUrl string `json:"uploadUrl"`
	ExpiresAt string `json:"expiresAt"`
}
