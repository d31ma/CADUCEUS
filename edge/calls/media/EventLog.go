package media

import (
	"io"
	"log/slog"
)

type EventLog struct {
	logger *slog.Logger
}

func NewEventLog(output io.Writer) EventLog {
	return EventLog{logger: slog.New(slog.NewJSONHandler(output, nil))}
}

func (log EventLog) Started(httpAddress string, mediaAddress string) {
	log.logger.Info("calls.started", "http", httpAddress, "media", mediaAddress)
}

func (log EventLog) Stopping() {
	log.logger.Info("calls.stopping")
}

func (log EventLog) Joined(seat Seat, devices int) {
	log.logger.Info("member.joined", log.callField(), seat.CallId, log.deviceField(), seat.DeviceId, "devices", devices)
}

func (log EventLog) Left(seat Seat, reason string) {
	log.logger.Info("member.left", log.callField(), seat.CallId, log.deviceField(), seat.DeviceId, "reason", reason)
}

func (log EventLog) Ended(callId string) {
	log.logger.Info("call.ended", log.callField(), callId)
}

func (EventLog) callField() string   { return "callId" }
func (EventLog) deviceField() string { return "deviceId" }
