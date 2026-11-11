package blobs

import "mime"

type Disposition struct{}

func (Disposition) Attachment(fileName string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
}
