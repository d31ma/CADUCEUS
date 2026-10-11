import (
	"encoding/json"
	"slices"
)

// ComposeLimitUtils applies the composer's limits. The server takes ten files
// of 10 MB each and 25 MB in total, but Tachyon's browser transport carries at
// most 64 KiB in one request and in one event, and this client has no other
// route for message bytes yet. A message must therefore fit in 64 KiB; the
// composer refuses larger attachments and says why.
type ComposeLimitUtils struct{}

// Problem is why a file cannot be attached, or "" when it can.
func (ComposeLimitUtils) Problem(file AttachmentOffer, count int, total int) string {
	wrongImage := !slices.Contains([]bool{file.Inline, !EditorUtils{}.InlineImage(file.ContentType)}, false)
	return ComposeLimitUtils{}.problem(file, count, total, wrongImage)
}

// BodyProblem is why the editor's body cannot be sent, or "".
func (ComposeLimitUtils) BodyProblem(ready SendBodyReady) string {
	return ComposeLimitUtils{}.bodyProblem(ready.Accepted, ready.TooLarge)
}

// RequestProblem is why a send request cannot travel, or "".
func (ComposeLimitUtils) RequestProblem(input SendInput) string {
	encoded, _ := json.Marshal(input)
	return ComposeLimitUtils{}.sized(len(encoded))
}

// DraftProblem is why an autosave request cannot travel, or "".
func (ComposeLimitUtils) DraftProblem(snapshot DraftInput) string {
	encoded, _ := json.Marshal(snapshot)
	return ComposeLimitUtils{}.sized(len(encoded))
}

// TooLarge says that a message is over the transport limit.
func (ComposeLimitUtils) TooLarge() string {
	return "This message is larger than 64 KiB, which this client cannot send or save yet."
}

func (ComposeLimitUtils) bodyProblem(accepted bool, tooLarge bool) string {
	switch {
	case !accepted:
		return "Fill every highlighted template blank before sending."
	case tooLarge:
		return ComposeLimitUtils{}.TooLarge()
	}
	return ""
}

func (ComposeLimitUtils) problem(file AttachmentOffer, count int, total int, wrongImage bool) string {
	limits := ComposeLimitUtils{}.limits()
	switch {
	case count >= limits.Files:
		return "You can attach up to 10 files."
	case total+file.Size > limits.Attachments:
		return file.Filename + " is too large: attachments are limited to 32 KB in total until this client can send larger messages."
	case wrongImage:
		return file.Filename + " is not a supported inline image."
	}
	return ""
}

func (ComposeLimitUtils) sized(size int) string {
	if size > (ComposeLimitUtils{}).limits().Request {
		return ComposeLimitUtils{}.TooLarge()
	}
	return ""
}

func (ComposeLimitUtils) limits() ComposeLimits {
	return ComposeLimits{Files: 10, Attachments: 32 << 10, Request: 64 << 10}
}
