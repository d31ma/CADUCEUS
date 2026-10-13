import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"
)

// Boundary sends one signed, replay-resistant frame to the Go owner of FYLO
// and the authentication engine, and reads its one-line answer. The owner
// listens on loopback only; the frame is signed with the shared bridge key.
type Boundary struct {
	Address string
	Secret  string
}

// FromSettings answers the boundary that the deployment configures.
func (Boundary) FromSettings() Boundary {
	settings := Settings{}
	return Boundary{Address: settings.Value("FYLO_BRIDGE_ADDRESS"), Secret: settings.Value("FYLO_BRIDGE_SECRET")}
}

// Call sends payload and answers the result of a successful operation.
func (boundary Boundary) Call(payload any, limit int, timeout time.Duration) (json.RawMessage, error) {
	frame, err := boundary.frame(payload)
	if err != nil {
		return nil, err
	}
	line, err := boundary.exchange(frame, limit, timeout)
	if err != nil {
		return nil, err
	}
	return boundary.result(line)
}

func (boundary Boundary) frame(payload any) ([]byte, error) {
	if !boundary.usable() {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary requires FYLO_BRIDGE_ADDRESS and FYLO_BRIDGE_SECRET"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return boundary.signed(body)
}

func (boundary Boundary) usable() bool {
	host, _, err := net.SplitHostPort(boundary.Address)
	return err == nil && (host == "127.0.0.1" || host == "::1") && len(boundary.Secret) >= boundary.minimumSecretBytes()
}

func (Boundary) minimumSecretBytes() int { return 32 }

func (boundary Boundary) signed(body []byte) ([]byte, error) {
	nonceBytes := make([]byte, boundary.nonceBytes())
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	timestamp := time.Now().UnixMilli()
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(boundary.Secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "." + nonce + "." + encoded))
	frame, err := json.Marshal(map[string]any{
		"version": 1, "timestamp": timestamp, "nonce": nonce, "payload": encoded,
		"signature": hex.EncodeToString(mac.Sum(nil)),
	})
	return append(frame, '\n'), err
}

func (Boundary) nonceBytes() int { return 24 }

func (boundary Boundary) exchange(frame []byte, limit int, timeout time.Duration) ([]byte, error) {
	connection, err := net.DialTimeout("tcp", boundary.Address, timeout)
	if err != nil {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary is unavailable"}
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))
	if _, err := connection.Write(frame); err != nil {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary is unavailable"}
	}
	return boundary.readLine(bufio.NewReaderSize(connection, boundary.readBuffer()), limit)
}

func (Boundary) readBuffer() int { return 64 * 1024 }

func (Boundary) readLine(reader *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		chunk, more, err := reader.ReadLine()
		if err != nil {
			return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary did not answer"}
		}
		line = append(line, chunk...)
		if len(line) > limit {
			return nil, &BoundaryError{Code: "EFRAME_RESPONSE_TOO_LARGE", Message: "The Go boundary sent an oversized response"}
		}
		if !more {
			return line, nil
		}
	}
}

func (Boundary) result(line []byte) (json.RawMessage, error) {
	var answer struct {
		Ok     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &answer); err != nil {
		return nil, &BoundaryError{Code: "EFRAME_RESPONSE_INVALID", Message: "The Go boundary sent malformed JSON"}
	}
	if answer.Ok {
		return answer.Result, nil
	}
	if answer.Error == nil {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "boundary operation failed"}
	}
	return nil, &BoundaryError{Code: strings.TrimSpace(answer.Error.Code), Message: answer.Error.Message}
}
