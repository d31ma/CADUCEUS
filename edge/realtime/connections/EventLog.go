package connections

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

func (log EventLog) Started(publicAddress string, managementAddress string) {
	log.logger.Info("gateway.started", "public", publicAddress, "management", managementAddress)
}

func (log EventLog) Stopping() {
	log.logger.Info("gateway.stopping")
}

func (log EventLog) Opened(connectionId string, kind string) {
	log.logger.Info("connection.opened", log.connectionField(), connectionId, "kind", kind)
}

func (log EventLog) Closed(connectionId string, code int) {
	log.logger.Info("connection.closed", log.connectionField(), connectionId, "code", code)
}

func (log EventLog) OriginRefused() {
	log.logger.Warn("origin.refused")
}

func (log EventLog) BackendFailed(connectionId string, call string, err error) {
	log.reportFailure(connectionId, call, err)
}

func (log EventLog) reportFailure(connectionId string, call string, err error) {
	if err == nil {
		return
	}
	log.logger.Warn("backend.failed", log.connectionField(), connectionId, "call", call, "error", err.Error())
}

func (EventLog) connectionField() string { return "connectionId" }
