import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// SourceFrames writes the export of one folder's exact RFC 822 sources. The
// body starts with the magic line, then holds one frame per message:
//
//	uint32be metadataLength | uint32be sourceLength | metadata JSON | source
//
// The browser assembles mboxrd or EML archives from the frames.
type SourceFrames struct {
	body bytes.Buffer
}

func (SourceFrames) magic() string { return "CADUCEUS-RFC822-FRAMES/1\n" }

func (SourceFrames) largestMetadata() int { return 16 * 1024 }

// HostedFolder answers a hosted folder name, or why it is invalid.
func (SourceFrames) HostedFolder(value string) (string, error) {
	if (TextValue{}).Trim(value) == "" {
		return "", errors.New("folder is required")
	}
	folder := strings.ToLower(TextValue{}.Trim(value))
	if (TextValue{}).Length(folder) > 60 || regexp.MustCompile(`[\x00-\x1f\x7f/\\]`).MatchString(folder) {
		return "", errors.New("folder is invalid")
	}
	return folder, nil
}

// ProviderFolder answers a connected-account folder name, or why it is
// invalid.
func (SourceFrames) ProviderFolder(value string) (string, error) {
	if (TextValue{}).Trim(value) == "" {
		return "", errors.New("folder is required")
	}
	folder := TextValue{}.Trim(value)
	if (TextValue{}).Length(folder) > 1024 || regexp.MustCompile(`[\x00-\x1f\x7f]`).MatchString(folder) {
		return "", errors.New("folder is invalid")
	}
	return folder, nil
}

// Add writes the frame of one message.
func (frames *SourceFrames) Add(email map[string]any, source []byte) error {
	if frames.body.Len() == 0 {
		frames.body.WriteString(frames.magic())
	}
	if int64(len(source)) > 0xffff_ffff {
		return errors.New("RFC 822 source exceeds the export frame limit")
	}
	metadata := []byte(TextValue{}.Json(map[string]any{
		"id": email["id"], "sender": email["sender"], "subject": email["subject"], "receivedAt": email["receivedAt"],
	}))
	if len(metadata) > frames.largestMetadata() {
		return errors.New("Export metadata exceeds its limit")
	}
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], uint32(len(metadata)))
	binary.BigEndian.PutUint32(header[4:8], uint32(len(source)))
	frames.body.Write(header)
	frames.body.Write(metadata)
	frames.body.Write(source)
	return nil
}

// Response answers the export.
func (frames *SourceFrames) Response() *http.Response {
	if frames.body.Len() == 0 {
		frames.body.WriteString(frames.magic())
	}
	header := http.Header{
		"Content-Type":             {"application/vnd.caduceus.rfc822-frames; version=1"},
		"Cache-Control":            {"no-store"},
		"X-Content-Type-Options":   {"nosniff"},
		"X-Caduceus-Export-Format": {"rfc822-source-frames-v1"},
	}
	content := frames.body.Bytes()
	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(bytes.NewReader(content)), ContentLength: int64(len(content))}
}
