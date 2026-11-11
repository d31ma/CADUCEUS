package blobs

type HeadBuffer struct {
	capacity int
	bytes    []byte
}

func (head *HeadBuffer) Write(data []byte) (int, error) {
	room := head.capacity - len(head.bytes)
	head.bytes = append(head.bytes, data[:min(room, len(data))]...)
	return len(data), nil
}

func (head *HeadBuffer) Bytes() []byte {
	return head.bytes
}
