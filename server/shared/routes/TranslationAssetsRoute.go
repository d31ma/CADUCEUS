import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// TranslationAssetsRoute answers /translation-assets: authenticated,
// allowlisted delivery of the checksum-pinned translation payload kept
// outside the client asset tree.
type TranslationAssetsRoute struct {
	Access *AccessHelper
}

// Read answers one asset the installed catalog lists, after checking its
// size against the catalog.
func (route TranslationAssetsRoute) Read(request *http.Request) *http.Response {
	if _, refusal := route.Access.Signed(request); refusal != nil {
		return refusal
	}
	requested, problem := route.assetPath(request.URL.Query()["path"])
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	configuredRoot := Settings{}.Value("TRANSLATION_ASSET_ROOT")
	if configuredRoot == "" {
		return Answer{}.Unavailable("Self-hosted translation assets are not installed")
	}
	root, err := filepath.Abs(configuredRoot)
	if err != nil {
		return Answer{}.Unavailable("Self-hosted translation assets are not installed")
	}
	catalogText, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		return Answer{}.Unavailable("Self-hosted translation assets are not installed")
	}
	var catalog any
	if json.Unmarshal(catalogText, &catalog) != nil {
		return Answer{}.Unavailable("Self-hosted translation asset catalog is invalid")
	}
	bytes, digest, listed := route.entry(catalog, requested)
	if !listed {
		return Answer{}.NotFound("Translation asset not found")
	}
	if (Settings{}).Value("CADUCEUS_TEST_REAL_TRANSLATION") == "true" {
		fmt.Fprintf(os.Stderr, "[translation-asset] %s (%d)\n", requested, bytes)
	}
	return route.serve(root, requested, bytes, digest)
}

func (route TranslationAssetsRoute) serve(root string, requested string, bytes int64, digest string) *http.Response {
	path := filepath.Join(root, requested)
	if !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return Answer{}.BadRequest("Invalid translation asset path")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != bytes {
		return Answer{}.Unavailable("Translation asset failed its catalog check")
	}
	file, err := os.Open(path)
	if err != nil {
		return Answer{}.Unavailable("Translation asset failed its catalog check")
	}
	header := http.Header{
		"Content-Type":           {route.contentType(requested)},
		"Content-Length":         {strconv.FormatInt(bytes, 10)},
		"Cache-Control":          {"private, max-age=31536000, immutable"},
		"Etag":                   {`"sha256-` + digest + `"`},
		"Vary":                   {"Authorization"},
		"X-Content-Type-Options": {"nosniff"},
	}
	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: file, ContentLength: bytes}
}

// entry answers the catalog's size and digest of the requested path.
func (TranslationAssetsRoute) entry(catalog any, requested string) (int64, string, bool) {
	object, _ := catalog.(map[string]any)
	files, _ := object["files"].([]any)
	for _, item := range files {
		file, _ := item.(map[string]any)
		bytes, isNumber := file["bytes"].(float64)
		digest, _ := file["sha256"].(string)
		safe := isNumber && bytes == float64(int64(bytes)) && bytes >= -9007199254740991 && bytes <= 9007199254740991
		if file["path"] == requested && safe && regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(digest) {
			return int64(bytes), digest, true
		}
	}
	return 0, "", false
}

// assetPath answers the requested path when it names a file under models/
// or runtime/ without leaving them.
func (TranslationAssetsRoute) assetPath(values []string) (string, string) {
	if len(values) == 0 || values[len(values)-1] == "" || (TextValue{}).Length(values[len(values)-1]) > 512 {
		return "", "Translation asset path is required"
	}
	normalized := strings.ReplaceAll(values[len(values)-1], "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, "../") || strings.Contains(normalized, "/..") || strings.Contains(normalized, "\x00") || !regexp.MustCompile(`^(?:models|runtime)/[A-Za-z0-9._/-]+$`).MatchString(normalized) {
		return "", "Invalid translation asset path"
	}
	return normalized, ""
}

func (TranslationAssetsRoute) contentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".wasm"):
		return "application/wasm"
	}
	return "application/octet-stream"
}
