//go:build responsive

package responsive

import (
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
)

type RealtimeFake struct{}

const (
	realtimeSocketPath = "/api/v1/realtime/socket"
	webSocketGuid      = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

func (fake RealtimeFake) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	key := request.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(writer, "expected a WebSocket upgrade", http.StatusBadRequest)
		return
	}
	connection, buffer, err := http.NewResponseController(writer).Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	_, _ = buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + fake.accept(key) + "\r\n\r\n")
	if buffer.Flush() != nil {
		return
	}
	_, _ = io.Copy(io.Discard, buffer)
}

func (fake RealtimeFake) accept(key string) string {
	digest := sha1.Sum([]byte(key + webSocketGuid))
	return base64.StdEncoding.EncodeToString(digest[:])
}
