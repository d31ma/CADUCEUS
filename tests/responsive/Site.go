//go:build responsive

package responsive

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Site struct {
	DistFolder        string
	SourceFolder      string
	ContentPolicy     string
	Profile           AppProfile
	server            *httptest.Server
	unknownApiCalls   []string
	apiCalls          []string
	unknownApiCallsMu sync.Mutex
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

const (
	blankPagePath    = "/__responsive/blank"
	identityApiPath  = "/api/v1/identity/"
	apiPath          = "/api/"
	indexFileName    = "index.html"
	blankPageContent = "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>blank</title></head><body></body></html>"
)

func StartSite(distFolder string, profile AppProfile) (*Site, error) {
	if _, err := os.Stat(filepath.Join(distFolder, indexFileName)); err != nil {
		return nil, fmt.Errorf("the built %s app is missing (%w); build it with `%s` or set MESAIL_DIST", profile.Name, err, profile.BuildCommand())
	}
	copied, err := copyBuild(distFolder)
	if err != nil {
		return nil, fmt.Errorf("copying %s: %w", distFolder, err)
	}
	indexHtml, err := os.ReadFile(filepath.Join(copied, indexFileName))
	if err != nil {
		return nil, err
	}
	site := &Site{DistFolder: copied, SourceFolder: distFolder, ContentPolicy: ServerContentPolicy(string(indexHtml)), Profile: profile}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	site.server = httptest.NewUnstartedServer(site.handler())
	site.server.Listener = listener
	site.server.Start()
	return site, nil
}

func ServerContentPolicy(indexHtml string) string {
	scriptSources := []string{"'self'", "'wasm-unsafe-eval'"}
	for _, script := range inlineScripts(indexHtml) {
		digest := sha256.Sum256([]byte(script))
		scriptSources = append(scriptSources, "'sha256-"+base64.StdEncoding.EncodeToString(digest[:])+"'")
	}
	return "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
		"script-src " + strings.Join(scriptSources, " ") + "; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"
}

func inlineScripts(indexHtml string) []string {
	scriptPattern := regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`)
	scripts := []string{}
	for _, match := range scriptPattern.FindAllStringSubmatch(indexHtml, -1) {
		if isExecutedInlineScript(match[1]) {
			scripts = append(scripts, match[2])
		}
	}
	return scripts
}

func isExecutedInlineScript(attributes string) bool {
	return !strings.Contains(attributes, "src=") && !strings.Contains(attributes, `type="application/json"`)
}

func (site *Site) Url(pagePath string) string {
	return site.server.URL + pagePath
}

func (site *Site) Origin() string {
	return site.server.URL
}

func (site *Site) UnknownApiCalls() []string {
	site.unknownApiCallsMu.Lock()
	defer site.unknownApiCallsMu.Unlock()
	return append([]string{}, site.unknownApiCalls...)
}

func (site *Site) Close() {
	site.server.Close()
	_ = os.RemoveAll(site.DistFolder)
}

func copyBuild(source string) (string, error) {
	target, err := os.MkdirTemp("", "mesail-responsive-dist-")
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(source, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(source, sourcePath)
		return copyEntry(sourcePath, filepath.Join(target, relative), entry)
	})
	return target, err
}

func copyEntry(sourcePath string, targetPath string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}
	if strings.HasSuffix(sourcePath, ".br") || strings.HasSuffix(sourcePath, ".gz") {
		return nil
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, content, 0o644)
}

func (site *Site) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(identityApiPath, NewIdentityFake(site.Profile))
	mux.HandleFunc(apiPath, site.otherApi)
	mux.HandleFunc(blankPagePath, site.blankPage)
	mux.HandleFunc("/", site.staticFile)
	return site.withSecurityHeaders(mux)
}

func (site *Site) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if site.ContentPolicy != "" {
			writer.Header().Set("Content-Security-Policy", site.ContentPolicy)
		}
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Cache-Control", "no-store")
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		site.recordApiCall(request, recorder.status)
	})
}

func (site *Site) recordApiCall(request *http.Request, status int) {
	if !strings.HasPrefix(request.URL.Path, apiPath) {
		return
	}
	site.unknownApiCallsMu.Lock()
	defer site.unknownApiCallsMu.Unlock()
	site.apiCalls = append(site.apiCalls, fmt.Sprintf("%s %s -> %d", request.Method, request.URL.Path, status))
}

func (site *Site) ApiCalls() []string {
	site.unknownApiCallsMu.Lock()
	defer site.unknownApiCallsMu.Unlock()
	return append([]string{}, site.apiCalls...)
}

func (site *Site) otherApi(writer http.ResponseWriter, request *http.Request) {
	answer, found := site.Profile.DataAnswer(request)
	if !found {
		site.unknownApi(writer, request)
		return
	}
	writeJson(writer, http.StatusOK, answer)
}

func (site *Site) unknownApi(writer http.ResponseWriter, request *http.Request) {
	site.unknownApiCallsMu.Lock()
	site.unknownApiCalls = append(site.unknownApiCalls, request.Method+" "+request.URL.Path)
	site.unknownApiCallsMu.Unlock()
	writeJson(writer, http.StatusNotFound, fakeApiError{Code: "not_found", Message: "not found"})
}

func (site *Site) blankPage(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(blankPageContent))
}

func (site *Site) staticFile(writer http.ResponseWriter, request *http.Request) {
	cleanPath := path.Clean("/" + request.URL.Path)
	filePath := filepath.Join(site.DistFolder, filepath.FromSlash(cleanPath))
	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		filePath = filepath.Join(filePath, indexFileName)
		info, err = os.Stat(filePath)
	}
	if err != nil && path.Ext(cleanPath) == "" {
		filePath = filepath.Join(site.DistFolder, indexFileName)
		info, err = os.Stat(filePath)
	}
	if err != nil || info.IsDir() {
		http.NotFound(writer, request)
		return
	}
	http.ServeFile(writer, request, filePath)
}
