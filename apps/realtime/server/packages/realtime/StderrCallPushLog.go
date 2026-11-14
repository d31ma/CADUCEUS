package realtime

import (
	"io"
	"log/slog"
)

type StderrCallPushLog struct {
	Output io.Writer
}

func (log StderrCallPushLog) PushFailed(domainId string, kind string, failure error) {
	slog.New(slog.NewJSONHandler(log.Output, nil)).Warn("realtime.call_push_failed", "domainId", domainId, "kind", kind, "error", failure.Error())
}
