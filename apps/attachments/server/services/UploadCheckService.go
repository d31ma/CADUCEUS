import (
	"cmp"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"mesail.dev/packages/blobs"
)

// @Service
type UploadCheckService struct {
}

func (service *UploadCheckService) Check(request CreateUploadRequest, limit int64) error {
	return cmp.Or(
		service.checkFileName(request.FileName), service.checkMediaType(request.MediaType), service.checkSizeGiven(request.Size),
		service.checkSizeLimit(request.Size, limit), service.checkSha256(request.Sha256),
	)
}

func (service *UploadCheckService) checkFileName(fileName string) error {
	if fileName == "" || len(fileName) > service.longestFileName() || !utf8.ValidString(fileName) || strings.ContainsAny(fileName, "/\\") || strings.ContainsFunc(fileName, unicode.IsControl) {
		return &InvalidUpload{Field: "fileName", Reason: "Give a file name of 1 to 255 bytes without slashes or control characters."}
	}
	return nil
}

func (service *UploadCheckService) checkMediaType(mediaType string) error {
	if !(blobs.MediaTypes{}).Valid(mediaType) {
		return &InvalidUpload{Field: "mediaType", Reason: "Give a media type such as image/png."}
	}
	return nil
}

func (service *UploadCheckService) checkSizeGiven(size int64) error {
	if size < 1 {
		return &InvalidUpload{Field: "size", Reason: "An attachment must have at least one byte."}
	}
	return nil
}

func (service *UploadCheckService) checkSizeLimit(size int64, limit int64) error {
	if size > limit {
		return &AttachmentTooLarge{Limit: limit}
	}
	return nil
}

func (service *UploadCheckService) checkSha256(sha256 string) error {
	if !regexp.MustCompile(service.sha256Pattern()).MatchString(sha256) {
		return &InvalidUpload{Field: "sha256", Reason: "Give the SHA-256 of the file as 64 lowercase hex characters."}
	}
	return nil
}

func (service *UploadCheckService) longestFileName() int {
	return 255
}

func (service *UploadCheckService) sha256Pattern() string {
	return `^[0-9a-f]{64}$`
}
