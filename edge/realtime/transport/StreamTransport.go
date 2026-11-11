package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type StreamTransport struct {
	writer       http.ResponseWriter
	control      *http.ResponseController
	writeTimeout time.Duration
}

func NewStreamTransport(writer http.ResponseWriter, writeTimeout time.Duration) StreamTransport {
	return StreamTransport{writer: writer, control: http.NewResponseController(writer), writeTimeout: writeTimeout}
}

func (transport StreamTransport) Open() error {
	headers := transport.writer.Header()
	headers.Set(Headers{}.ContentType(), "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("X-Accel-Buffering", "no")
	transport.writer.WriteHeader(http.StatusOK)
	return transport.flush()
}

func (transport StreamTransport) Send(event []byte) error {
	return transport.write(fmt.Sprintf("data: %s\n\n", event))
}

func (transport StreamTransport) Heartbeat() error {
	return transport.write(": keep-alive\n\n")
}

func (transport StreamTransport) Close(code int) {
	encoded, _ := json.Marshal(CloseEvent{Code: code})
	_ = transport.write(fmt.Sprintf("event: close\ndata: %s\n\n", encoded))
}

func (transport StreamTransport) write(text string) error {
	_ = transport.control.SetWriteDeadline(time.Now().Add(transport.writeTimeout))
	_, err := io.WriteString(transport.writer, text)
	if err != nil {
		return err
	}
	return transport.flush()
}

func (transport StreamTransport) flush() error {
	return transport.control.Flush()
}
