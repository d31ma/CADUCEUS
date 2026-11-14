package transport

import (
	"net/http"

	"mesail.dev/edge/calls/desk"
)

type Routes struct {
	calls desk.CallDesk
}

func NewRoutes(calls desk.CallDesk) Routes {
	return Routes{calls: calls}
}

func (routes Routes) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/calls/{callId}/offer", OfferHandler{calls: routes.calls})
	mux.Handle("POST /api/v1/calls/{callId}/candidates", CandidatesHandler{calls: routes.calls})
	mux.Handle("POST /api/v1/calls/{callId}/leave", LeaveHandler{calls: routes.calls})
	mux.Handle("GET /api/v1/calls/health", HealthHandler{})
	return mux
}
