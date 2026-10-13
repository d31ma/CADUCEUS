package httpserver

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func staticHandler(root string, assets []string) http.Handler {
	filesystem := http.Dir(root)
	fileServer := http.FileServer(filesystem)
	allowed := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		allowed[asset] = struct{}{}
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			problem(response, request, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed")
			return
		}
		clean := path.Clean("/" + request.URL.Path)
		name := strings.TrimPrefix(clean, "/")
		if containsHiddenSegment(name) {
			problem(response, request, http.StatusNotFound, "resource_not_found", "Static asset not found")
			return
		}
		if name == "" {
			name = "index.html"
		}
		served, err := serveExisting(filesystem, fileServer, response, request, name, allowed)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			problem(response, request, http.StatusInternalServerError, "internal_error", "Static asset unavailable")
			return
		}
		if served {
			return
		}
		if path.Ext(name) != "" {
			problem(response, request, http.StatusNotFound, "resource_not_found", "Static asset not found")
			return
		}

		indexRequest := request.Clone(request.Context())
		indexRequest.URL.Path = "/"
		setStaticCache(response.Header(), "index.html")
		fileServer.ServeHTTP(response, indexRequest)
	})
}

func containsHiddenSegment(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func serveExisting(
	filesystem http.FileSystem,
	fileServer http.Handler,
	response http.ResponseWriter,
	request *http.Request,
	name string,
	allowed map[string]struct{},
) (bool, error) {
	if _, exists := allowed[name]; !exists {
		indexName := path.Join(name, "index.html")
		if _, indexExists := allowed[indexName]; !indexExists {
			return false, fs.ErrNotExist
		}
	}
	file, err := filesystem.Open(name)
	if err != nil {
		return false, err
	}
	info, err := file.Stat()
	_ = file.Close()
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		setStaticCache(response.Header(), name)
		fileServer.ServeHTTP(response, request)
		return true, nil
	}
	index, err := filesystem.Open(path.Join(name, "index.html"))
	if err != nil {
		return false, err
	}
	_ = index.Close()
	setStaticCache(response.Header(), "index.html")
	fileServer.ServeHTTP(response, request)
	return true, nil
}

func setStaticCache(header http.Header, name string) {
	if strings.HasSuffix(strings.ToLower(name), ".html") {
		header.Set("Cache-Control", "public, max-age=0, must-revalidate")
		return
	}
	header.Set("Cache-Control", "public, max-age=86400, immutable")
}
