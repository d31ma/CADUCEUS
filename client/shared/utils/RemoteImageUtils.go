import (
	"fmt"
	"strings"
)

// RemoteImageUtils accepts only bounded raster answers from the exact-message
// image proxy and words the reader's privacy notice.
type RemoteImageUtils struct{}

func (RemoteImageUtils) Summary(blocked int, removed int) string {
	images := TextUtils{}.Choose(blocked == 1, "image is", "images are")
	pixels := TextUtils{}.Choose(removed == 1, "pixel was", "pixels were")
	summaries := map[[2]bool]string{
		{true, true}:  fmt.Sprintf("%d remote %s blocked and %d tracking %s removed.", blocked, images, removed, pixels),
		{true, false}: fmt.Sprintf("%d remote %s blocked to protect your privacy.", blocked, images),
		{false, true}: fmt.Sprintf("%d tracking %s removed.", removed, pixels),
	}
	return summaries[[2]bool{blocked > 0, removed > 0}]
}

func (RemoteImageUtils) Failure(failures int) string {
	return TextUtils{}.Choose(failures == 0, "", fmt.Sprintf("%d remote %s not be loaded safely.", failures, TextUtils{}.Choose(failures == 1, "image could", "images could")))
}

func (RemoteImageUtils) Failures(images ProxyImages, failure error, requested int) int {
	return RemoteImageUtils{}.failures(images, failure, requested)
}

func (RemoteImageUtils) Keep(sources map[string]string, images []ProxyImage) {
	RemoteImageUtils{}.keepAll(sources, images)
}

func (RemoteImageUtils) keepAll(sources map[string]string, images []ProxyImage) {
	for _, image := range images {
		RemoteImageUtils{}.keep(sources, image, RemoteImageUtils{}.source(image))
	}
}

func (RemoteImageUtils) failures(images ProxyImages, failure error, requested int) int {
	if failure != nil {
		return requested
	}
	return max(0, images.Failed) + RemoteImageUtils{}.rejected(images.Images)
}

func (RemoteImageUtils) rejected(images []ProxyImage) int {
	count := 0
	for _, image := range images {
		count += RemoteImageUtils{}.rejectedCount(RemoteImageUtils{}.source(image))
	}
	return count
}

func (RemoteImageUtils) rejectedCount(source string) int {
	if source == "" {
		return 1
	}
	return 0
}

func (RemoteImageUtils) keep(sources map[string]string, image ProxyImage, source string) {
	if source != "" {
		sources[image.Url] = source
	}
}

func (RemoteImageUtils) maximumBytes() float64 { return 4194304 }

func (RemoteImageUtils) base64Ratio() float64 { return 0.75 }

func (RemoteImageUtils) padding() float64 { return 2 }

func (RemoteImageUtils) source(image ProxyImage) string {
	contentType := strings.ToLower(image.ContentType)
	limit := RemoteImageUtils{}.maximumBytes()
	source := "data:" + contentType + ";base64," + image.ContentBase64
	safe := StyleProtectionUtils{}.IsSafeEmbeddedSource(source)
	raster := AttachmentContentUtils{}.IsRasterImage(contentType)
	decoded := float64(len(image.ContentBase64)) * RemoteImageUtils{}.base64Ratio()
	ceiling := limit + RemoteImageUtils{}.padding()
	if !raster || image.Size < 1 || image.Size > limit || decoded > ceiling || !safe {
		return ""
	}
	return source
}
