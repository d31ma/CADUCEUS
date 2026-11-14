package oidc

import "time"

type PasskeyRecord struct {
	PasskeyId    string `json:"passkeyId"`
	CredentialId string `json:"credentialId"`
	Label        string `json:"label"`
	CreatedAt    int64  `json:"createdAt"`
	LastUsedAt   *int64 `json:"lastUsedAt"`
}

func (record PasskeyRecord) Passkey() Passkey {
	return Passkey{PasskeyId: record.PasskeyId, Label: record.Label, CreatedAt: record.time(record.CreatedAt), LastUsedAt: record.lastUsed()}
}

func (record PasskeyRecord) lastUsed() string {
	if record.LastUsedAt == nil {
		return ""
	}
	return record.time(*record.LastUsedAt)
}

func (PasskeyRecord) time(milliseconds int64) string {
	return time.UnixMilli(milliseconds).UTC().Format(time.RFC3339)
}
