package blobs

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type LocalHandler struct {
	folder LocalFolder
	prefix string
	now    func() time.Time
}

func NewLocalHandler(folder LocalFolder, prefix string, now func() time.Time) LocalHandler {
	return LocalHandler{folder: folder, prefix: prefix, now: now}
}

func (handler LocalHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	grant, err := handler.grant(request)
	handler.action(grant.Method, err)(writer, request, grant)
}

func (handler LocalHandler) grant(request *http.Request) (Grant, error) {
	domainId, name, _ := strings.Cut(strings.TrimPrefix(request.URL.Path, handler.prefix+"/"), "/")
	return handler.folder.Grant(request.Method, Key{DomainId: domainId, Name: name}, request.URL.Query(), handler.now())
}

func (handler LocalHandler) action(method string, err error) func(http.ResponseWriter, *http.Request, Grant) {
	if err != nil {
		return func(writer http.ResponseWriter, _ *http.Request, _ Grant) { handler.refuse(writer, err) }
	}
	return handler.actions()[method]
}

func (handler LocalHandler) actions() map[string]func(http.ResponseWriter, *http.Request, Grant) {
	return map[string]func(http.ResponseWriter, *http.Request, Grant){http.MethodPut: handler.store, http.MethodGet: handler.send}
}

func (handler LocalHandler) store(writer http.ResponseWriter, request *http.Request, grant Grant) {
	err := handler.folder.Write(grant.Key, request.Body, grant.Size)
	if err != nil {
		handler.refuse(writer, err)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (handler LocalHandler) send(writer http.ResponseWriter, _ *http.Request, grant Grant) {
	reader, err := handler.folder.Read(grant.Key)
	if err != nil {
		handler.refuse(writer, err)
		return
	}
	defer reader.Close()
	handler.writeHeaders(writer.Header(), grant)
	io.Copy(writer, reader)
}

func (handler LocalHandler) writeHeaders(header http.Header, grant Grant) {
	header.Set("Content-Type", grant.MediaType)
	header.Set("Content-Disposition", Disposition{}.Attachment(grant.FileName))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Content-Security-Policy", "default-src 'none'; sandbox")
}

func (handler LocalHandler) refuse(writer http.ResponseWriter, err error) {
	status := handler.status(err)
	http.Error(writer, http.StatusText(status), status)
}

func (LocalHandler) status(err error) int {
	var known HttpStatus
	if errors.As(err, &known) {
		return known.HttpStatus()
	}
	return http.StatusInternalServerError
}
