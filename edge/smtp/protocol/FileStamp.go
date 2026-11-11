package protocol

import "time"

type FileStamp struct {
	CertificateTime time.Time
	KeyTime         time.Time
}
