package mailchecks

import "mesail.dev/packages/mime"

type HeaderParser interface {
	Headers(raw []byte) (mime.Headers, error)
}
