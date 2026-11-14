package media

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/pion/webrtc/v4"
)

type SlotChannel struct {
	lock    sync.Mutex
	channel *webrtc.DataChannel
	latest  []byte
	sent    []byte
}

func (holder *SlotChannel) Attach(channel *webrtc.DataChannel) {
	holder.attach(channel)
}

func (holder *SlotChannel) attach(channel *webrtc.DataChannel) {
	if channel.Label() != holder.label() {
		return
	}
	holder.keep(channel)
	channel.OnOpen(holder.flush)
}

func (holder *SlotChannel) Publish(message SlotMessage) {
	text, _ := json.Marshal(message)
	holder.lock.Lock()
	holder.latest = text
	holder.lock.Unlock()
	holder.flush()
}

func (holder *SlotChannel) keep(channel *webrtc.DataChannel) {
	holder.lock.Lock()
	defer holder.lock.Unlock()
	holder.channel = channel
}

func (holder *SlotChannel) flush() {
	holder.lock.Lock()
	defer holder.lock.Unlock()
	if !holder.ready() {
		return
	}
	holder.sent = holder.latest
	_ = holder.channel.SendText(string(holder.latest))
}

func (holder *SlotChannel) ready() bool {
	return holder.channel != nil && holder.channel.ReadyState() == webrtc.DataChannelStateOpen && !bytes.Equal(holder.latest, holder.sent)
}

func (*SlotChannel) label() string {
	return "calls"
}
