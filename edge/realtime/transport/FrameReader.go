package transport

import (
	"context"
	"io"

	"github.com/coder/websocket"
	"mesail.dev/edge/realtime/connections"
)

type FrameReader struct {
	connection *websocket.Conn
}

func NewFrameReader(connection *websocket.Conn) FrameReader {
	connection.SetReadLimit(FrameReader{}.MaxFrameBytes() + 1)
	return FrameReader{connection: connection}
}

func (reader FrameReader) Next() (string, int) {
	messageType, source, err := reader.connection.Reader(context.Background())
	if err != nil {
		return "", reader.peerCode(err)
	}
	body, err := io.ReadAll(io.LimitReader(source, reader.MaxFrameBytes()+1))
	if err != nil {
		return "", reader.peerCode(err)
	}
	return string(body), reader.firstProblem(reader.typeProblem(messageType), reader.sizeProblem(body))
}

func (FrameReader) MaxFrameBytes() int64 { return 16384 }

func (FrameReader) peerCode(err error) int {
	code := int(websocket.CloseStatus(err))
	if code < 0 {
		return connections.CloseCodes{}.Broken()
	}
	return code
}

func (FrameReader) typeProblem(messageType websocket.MessageType) int {
	if messageType != websocket.MessageText {
		return connections.CloseCodes{}.UnsupportedData()
	}
	return 0
}

func (reader FrameReader) sizeProblem(body []byte) int {
	if int64(len(body)) > reader.MaxFrameBytes() {
		return connections.CloseCodes{}.MessageTooBig()
	}
	return 0
}

func (FrameReader) firstProblem(typeProblem int, sizeProblem int) int {
	if typeProblem != 0 {
		return typeProblem
	}
	return sizeProblem
}
