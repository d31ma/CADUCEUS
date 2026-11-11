package attachments

import (
	"cmp"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"mesail.dev/packages/blobs"
)

type UploadRules struct{}

func (rules UploadRules) Check(request UploadRequest, limit int64) error {
	return cmp.Or(
		rules.checkFileName(request.FileName), rules.checkMediaType(request.MediaType), rules.checkSizeGiven(request.Size),
		rules.checkSizeLimit(request.Size, limit), rules.checkSha256(request.Sha256),
	)
}

func (rules UploadRules) checkFileName(fileName string) error {
	if fileName == "" || len(fileName) > rules.longestFileName() || !utf8.ValidString(fileName) || strings.ContainsAny(fileName, "/\\") || strings.ContainsFunc(fileName, unicode.IsControl) {
		return Invalid{Field: "fileName", Reason: "Give a file name of 1 to 255 bytes without slashes or control characters."}
	}
	return nil
}

func (UploadRules) checkMediaType(mediaType string) error {
	if !(blobs.MediaTypes{}).Valid(mediaType) {
		return Invalid{Field: "mediaType", Reason: "Give a media type such as image/png."}
	}
	return nil
}

func (UploadRules) checkSizeGiven(size int64) error {
	if size < 1 {
		return Invalid{Field: "size", Reason: "An attachment must have at least one byte."}
	}
	return nil
}

func (UploadRules) checkSizeLimit(size int64, limit int64) error {
	if size > limit {
		return blobs.TooLarge{Limit: limit}
	}
	return nil
}

func (rules UploadRules) checkSha256(sha256 string) error {
	if !regexp.MustCompile(rules.sha256Pattern()).MatchString(sha256) {
		return Invalid{Field: "sha256", Reason: "Give the SHA-256 of the file as 64 lowercase hex characters."}
	}
	return nil
}

func (UploadRules) longestFileName() int {
	return 255
}

func (UploadRules) sha256Pattern() string {
	return `^[0-9a-f]{64}$`
}
