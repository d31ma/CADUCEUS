// Package bridgeclient sends signed frames to the Go FYLO owner
// (`caduceus fylo-bridge`) from another process.
//
// The frame is the one internal/platform/fylobridge verifies and that
// tests/support/legacy-server/deps/fylo.mjs and server/shared/boundary
// (Boundary.go) send: one JSON line {version, timestamp, nonce, payload,
// signature} where payload is the base64url request and signature is the
// hex HMAC-SHA256 of "timestamp.nonce.payload". server/ is compiled by
// Tachyon as package main and cannot import this package, so the two
// implementations must be kept in step.
package bridgeclient

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// Error is a failed bridge or FYLO operation.
type Error struct {
	Code    string
	Message string
}

func (err *Error) Error() string {
	if err.Code == "" {
		return err.Message
	}
	return err.Code + ": " + err.Message
}

// Client talks to one bridge.
type Client struct {
	Address          string
	Secret           string
	Timeout          time.Duration
	MaxResponseBytes int
}

// New answers a client with the JavaScript defaults (10 s per request).
func New(address, secret string) *Client {
	return &Client{Address: address, Secret: secret, Timeout: 10 * time.Second, MaxResponseBytes: 64 << 20}
}

// Call sends one operation and answers its result.
func (client *Client) Call(ctx context.Context, request map[string]any) (json.RawMessage, error) {
	if client.Secret == "" {
		return nil, errors.New("FYLO bridge secret is unavailable")
	}
	host, _, err := net.SplitHostPort(client.Address)
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		return nil, errors.New("FYLO_BRIDGE_ADDRESS must be an explicit loopback host and TCP port")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	frame, err := client.frame(payload)
	if err != nil {
		return nil, err
	}
	timeout := client.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", client.Address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}
	limit := client.MaxResponseBytes
	if limit <= 0 {
		limit = 64 << 20
	}
	reader := bufio.NewReaderSize(conn, 64*1024)
	var line []byte
	for {
		chunk, more, err := reader.ReadLine()
		if err != nil {
			var timeoutErr net.Error
			if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
				return nil, &Error{Code: "ETIMEOUT", Message: "FYLO bridge timed out"}
			}
			return nil, err
		}
		line = append(line, chunk...)
		if len(line) > limit {
			return nil, &Error{Code: "EFRAME_RESPONSE_TOO_LARGE", Message: fmt.Sprintf("FYLO bridge response exceeds %d bytes", limit)}
		}
		if !more {
			break
		}
	}
	var answer struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &answer); err != nil {
		return nil, &Error{Code: "EFRAME_RESPONSE_INVALID", Message: "FYLO bridge returned malformed JSON"}
	}
	if !answer.OK {
		if answer.Error == nil {
			return nil, &Error{Message: "fylo error"}
		}
		return nil, &Error{Code: answer.Error.Code, Message: answer.Error.Message}
	}
	return answer.Result, nil
}

func (client *Client) frame(payload []byte) ([]byte, error) {
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	timestamp := time.Now().UnixMilli()
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(client.Secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "." + nonce + "." + encoded))
	frame, err := json.Marshal(struct {
		Version   int    `json:"version"`
		Timestamp int64  `json:"timestamp"`
		Nonce     string `json:"nonce"`
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}{1, timestamp, nonce, encoded, hex.EncodeToString(mac.Sum(nil))})
	if err != nil {
		return nil, err
	}
	return append(frame, '\n'), nil
}
